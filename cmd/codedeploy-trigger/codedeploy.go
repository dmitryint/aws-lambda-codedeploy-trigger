package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/dmitryint/aws-lambda-codedeploy-trigger/pkg/deploy"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	"github.com/aws/aws-sdk-go-v2/service/codedeploy/types"
)

type codeDeployAPI interface {
	CreateDeployment(ctx context.Context, params *codedeploy.CreateDeploymentInput, optFns ...func(*codedeploy.Options)) (*codedeploy.CreateDeploymentOutput, error)
	GetDeployment(ctx context.Context, params *codedeploy.GetDeploymentInput, optFns ...func(*codedeploy.Options)) (*codedeploy.GetDeploymentOutput, error)
}

func shouldWait(event deploy.Event) bool {
	return event.Wait == nil || *event.Wait
}

func waitForDeployment(ctx context.Context, svc codeDeployAPI, deploymentID string, interval time.Duration) (*types.DeploymentInfo, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var last *types.DeploymentInfo
	for {
		out, err := svc.GetDeployment(ctx, &codedeploy.GetDeploymentInput{
			DeploymentId: aws.String(deploymentID),
		})
		if err != nil && ctx.Err() != nil {
			return last, deploymentTimeoutError(ctx, deploymentID, last)
		}
		if err != nil {
			return nil, fmt.Errorf("get deployment %s: %w", deploymentID, err)
		}

		info := out.DeploymentInfo
		last = info
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
			return info, deploymentTimeoutError(ctx, deploymentID, info)
		case <-ticker.C:
		}
	}
}

func deploymentTimeoutError(ctx context.Context, deploymentID string, last *types.DeploymentInfo) error {
	status := "unknown"
	if last != nil {
		status = string(last.Status)
	}
	return fmt.Errorf("context canceled while waiting for deployment %s (last status: %s): %w", deploymentID, status, ctx.Err())
}

func runCodeDeploy(ctx context.Context, svc codeDeployAPI, event deploy.Event, interval time.Duration) (deploy.Result, error) {
	if event.ApplicationName == nil || *event.ApplicationName == "" {
		return deploy.Result{}, errors.New("applicationName is required")
	}
	if event.DeploymentGroupName == nil || *event.DeploymentGroupName == "" {
		return deploy.Result{}, errors.New("deploymentGroupName is required")
	}
	if event.Revision == nil {
		return deploy.Result{}, errors.New("revision is required")
	}

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

	if !shouldWait(event) {
		return res, nil
	}

	info, waitErr := waitForDeployment(ctx, svc, res.DeploymentID, interval)
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
