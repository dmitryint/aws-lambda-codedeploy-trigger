package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/dmitryint/aws-lambda-codedeploy-trigger/pkg/deploy"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	"github.com/aws/aws-sdk-go-v2/service/codedeploy/types"
)

const (
	defaultPollInterval = 15 * time.Second
	pollIntervalEnv     = "POLL_INTERVAL_SECONDS"
)

// revision is overridden at link time via -ldflags "-X main.revision=...".
var revision = "dev"

func pollInterval() time.Duration {
	raw, ok := os.LookupEnv(pollIntervalEnv)
	if !ok {
		return defaultPollInterval
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultPollInterval
	}
	return time.Duration(n) * time.Second
}

func waitForDeployment(ctx context.Context, svc *codedeploy.Client, deploymentID string, interval time.Duration) (*types.DeploymentInfo, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		out, err := svc.GetDeployment(ctx, &codedeploy.GetDeploymentInput{
			DeploymentId: aws.String(deploymentID),
		})
		if err != nil {
			return nil, fmt.Errorf("get deployment %s: %w", deploymentID, err)
		}

		info := out.DeploymentInfo
		log.Printf("deployment %s status: %s", deploymentID, info.Status)

		switch info.Status {
		case types.DeploymentStatusSucceeded:
			return info, nil
		case types.DeploymentStatusFailed, types.DeploymentStatusStopped:
			reason := ""
			if info.ErrorInformation != nil && info.ErrorInformation.Message != nil {
				reason = ": " + *info.ErrorInformation.Message
			}
			return info, fmt.Errorf("deployment %s ended with status %s%s", deploymentID, info.Status, reason)
		}

		select {
		case <-ctx.Done():
			// Surface a structured error including the deployment ID so the
			// caller (e.g. Terraform) can investigate the in-flight deployment.
			return info, fmt.Errorf("context cancelled while waiting for deployment %s (last status: %s): %w", deploymentID, info.Status, ctx.Err())
		case <-ticker.C:
		}
	}
}

func handleRequest(ctx context.Context, event deploy.Event) (deploy.Result, error) {
	if event.ApplicationName == nil || *event.ApplicationName == "" {
		return deploy.Result{}, errors.New("applicationName is required")
	}
	if event.DeploymentGroupName == nil || *event.DeploymentGroupName == "" {
		return deploy.Result{}, errors.New("deploymentGroupName is required")
	}
	if event.Revision == nil {
		return deploy.Result{}, errors.New("revision is required")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return deploy.Result{}, fmt.Errorf("load AWS SDK config: %w", err)
	}

	svc := codedeploy.NewFromConfig(cfg)

	out, err := svc.CreateDeployment(ctx, &codedeploy.CreateDeploymentInput{
		ApplicationName:               event.ApplicationName,
		DeploymentGroupName:           event.DeploymentGroupName,
		Revision:                      event.Revision,
		Description:                   event.Description,
		IgnoreApplicationStopFailures: aws.ToBool(event.IgnoreApplicationStopFailures),
		AutoRollbackConfiguration:     event.AutoRollbackConfiguration,
	})
	if err != nil {
		return deploy.Result{}, fmt.Errorf("create deployment: %w", err)
	}

	res := deploy.Result{
		DeploymentID: aws.ToString(out.DeploymentId),
		Status:       string(types.DeploymentStatusCreated),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	wait := true
	if event.Wait != nil {
		wait = *event.Wait
	}
	if !wait {
		return res, nil
	}

	info, waitErr := waitForDeployment(ctx, svc, res.DeploymentID, pollInterval())
	if info != nil {
		res.Status = string(info.Status)
		if info.CompleteTime != nil {
			res.CompletedAt = info.CompleteTime.UTC().Format(time.RFC3339)
		}
	}
	if waitErr != nil {
		return res, waitErr
	}
	return res, nil
}

func main() {
	log.Printf("aws-lambda-codedeploy-trigger %s", revision)
	lambda.Start(handleRequest)
}
