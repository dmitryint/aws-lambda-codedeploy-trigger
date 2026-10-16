package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/dmitryint/aws-lambda-codedeploy-trigger/pkg/deploy"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

const (
	primaryDeploymentStatus = "PRIMARY"
	maxDescribeMisses       = 4
)

type ecsAPI interface {
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	UpdateService(ctx context.Context, params *ecs.UpdateServiceInput, optFns ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error)
	DescribeServiceDeployments(ctx context.Context, params *ecs.DescribeServiceDeploymentsInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServiceDeploymentsOutput, error)
}

func runECS(ctx context.Context, svc ecsAPI, event deploy.Event, interval time.Duration) (deploy.Result, error) {
	cluster := aws.ToString(event.Cluster)
	service := aws.ToString(event.Service)
	if cluster == "" {
		return deploy.Result{}, errors.New("cluster is required")
	}
	if aws.ToString(event.TaskDefinition) == "" && !aws.ToBool(event.ForceNewDeployment) {
		return deploy.Result{}, errors.New("taskDefinition is required unless forceNewDeployment is true")
	}

	before, err := describeService(ctx, svc, cluster, service)
	if err != nil {
		return deploy.Result{}, err
	}
	if dc := before.DeploymentController; dc != nil && dc.Type != types.DeploymentControllerTypeEcs {
		return deploy.Result{}, fmt.Errorf("service %s uses the %s deployment controller, the ECS trigger requires %s", service, dc.Type, types.DeploymentControllerTypeEcs)
	}

	out, err := svc.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:            event.Cluster,
		Service:            event.Service,
		TaskDefinition:     event.TaskDefinition,
		ForceNewDeployment: aws.ToBool(event.ForceNewDeployment),
	})
	if err != nil {
		return deploy.Result{}, fmt.Errorf("update service %s: %w", service, err)
	}

	res := deploy.Result{
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Trigger:   deploy.TriggerECS,
	}

	after := out.Service
	if after == nil {
		if after, err = describeService(ctx, svc, cluster, service); err != nil {
			return res, err
		}
	}

	arn, noOp, err := startedServiceDeployment(ctx, svc, cluster, service, before, after, interval)
	if err != nil {
		return res, err
	}
	res.NoOp = noOp
	res.DeploymentID = arn
	res.ServiceDeploymentArn = arn
	if noOp {
		log.Printf("service %s: UpdateService started no new deployment (current service deployment: %q)", service, arn)
	} else {
		res.Status = string(types.ServiceDeploymentStatusPending)
		log.Printf("service %s: UpdateService started service deployment %s", service, arn)
	}

	if arn == "" || !event.ShouldWait() {
		return res, nil
	}

	sd, waitErr := waitForServiceDeployment(ctx, svc, arn, interval)
	applyServiceDeployment(&res, sd)
	if waitErr != nil && noOp && sd != nil && deploymentFailed(sd.Status) {
		return res, fmt.Errorf("UpdateService started no new deployment and the current service deployment did not succeed, set forceNewDeployment to redeploy: %w", waitErr)
	}
	return res, waitErr
}

func describeService(ctx context.Context, svc ecsAPI, cluster, service string) (*types.Service, error) {
	out, err := svc.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []string{service},
	})
	if err != nil {
		return nil, fmt.Errorf("describe service %s: %w", service, err)
	}
	if len(out.Services) == 0 {
		return nil, fmt.Errorf("describe service %s in cluster %s: %s", service, cluster, failureText(out.Failures))
	}
	return &out.Services[0], nil
}

func startedServiceDeployment(ctx context.Context, svc ecsAPI, cluster, service string, before, after *types.Service, interval time.Duration) (arn string, noOp bool, err error) {
	beforeArn := aws.ToString(before.CurrentServiceDeployment)
	if afterArn := aws.ToString(after.CurrentServiceDeployment); afterArn != "" && afterArn != beforeArn {
		return afterArn, false, nil
	}
	if primaryDeploymentID(after) == primaryDeploymentID(before) {
		return beforeArn, true, nil
	}

	arn, err = awaitNewServiceDeployment(ctx, svc, cluster, service, beforeArn, interval)
	return arn, false, err
}

func primaryDeploymentID(s *types.Service) string {
	for _, d := range s.Deployments {
		if aws.ToString(d.Status) == primaryDeploymentStatus {
			return aws.ToString(d.Id)
		}
	}
	return ""
}

func awaitNewServiceDeployment(ctx context.Context, svc ecsAPI, cluster, service, previousArn string, interval time.Duration) (string, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		s, err := describeService(ctx, svc, cluster, service)
		if err != nil {
			return "", err
		}
		if arn := aws.ToString(s.CurrentServiceDeployment); arn != "" && arn != previousArn {
			return arn, nil
		}
		log.Printf("service %s: waiting for the started service deployment to be reported", service)

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context canceled while waiting for service %s to report the service deployment started by UpdateService: %w", service, ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitForServiceDeployment(ctx context.Context, svc ecsAPI, arn string, interval time.Duration) (*types.ServiceDeployment, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var last *types.ServiceDeployment
	misses := 0
	for {
		sd, failures, err := describeServiceDeployment(ctx, svc, arn)
		switch {
		case err != nil && ctx.Err() != nil:
			return last, serviceDeploymentTimeoutError(ctx, arn, last)
		case err != nil:
			return last, err
		case sd == nil:
			misses++
			if misses >= maxDescribeMisses {
				return last, fmt.Errorf("describe service deployment %s: %s", arn, failureText(failures))
			}
			log.Printf("service deployment %s not visible yet: %s", arn, failureText(failures))
		default:
			misses = 0
			last = sd
			log.Printf("service deployment %s status: %s, lifecycle stage: %s", arn, sd.Status, sd.LifecycleStage)
			if sd.Status == types.ServiceDeploymentStatusSuccessful {
				return sd, nil
			}
			if deploymentFailed(sd.Status) {
				return sd, fmt.Errorf("service deployment %s ended with status %s (status reason: %s, last lifecycle stage: %s)",
					arn, sd.Status, aws.ToString(sd.StatusReason), sd.LifecycleStage)
			}
		}

		select {
		case <-ctx.Done():
			return last, serviceDeploymentTimeoutError(ctx, arn, last)
		case <-ticker.C:
		}
	}
}

func deploymentFailed(status types.ServiceDeploymentStatus) bool {
	switch status {
	case types.ServiceDeploymentStatusRollbackSuccessful, types.ServiceDeploymentStatusRollbackFailed, types.ServiceDeploymentStatusStopped:
		return true
	default:
		return false
	}
}

func describeServiceDeployment(ctx context.Context, svc ecsAPI, arn string) (*types.ServiceDeployment, []types.Failure, error) {
	out, err := svc.DescribeServiceDeployments(ctx, &ecs.DescribeServiceDeploymentsInput{
		ServiceDeploymentArns: []string{arn},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("describe service deployment %s: %w", arn, err)
	}
	for i := range out.ServiceDeployments {
		if aws.ToString(out.ServiceDeployments[i].ServiceDeploymentArn) == arn {
			return &out.ServiceDeployments[i], nil, nil
		}
	}
	return nil, out.Failures, nil
}

func serviceDeploymentTimeoutError(ctx context.Context, arn string, last *types.ServiceDeployment) error {
	status, stage := "unknown", "unknown"
	if last != nil {
		status, stage = string(last.Status), string(last.LifecycleStage)
	}
	return fmt.Errorf("context canceled while waiting for service deployment %s (last status: %s, last lifecycle stage: %s): %w", arn, status, stage, ctx.Err())
}

func applyServiceDeployment(res *deploy.Result, sd *types.ServiceDeployment) {
	if sd == nil {
		return
	}
	res.Status = string(sd.Status)
	res.LifecycleStage = string(sd.LifecycleStage)
	res.StatusReason = aws.ToString(sd.StatusReason)
	if sd.CreatedAt != nil {
		res.CreatedAt = sd.CreatedAt.UTC().Format(time.RFC3339)
	}
	switch {
	case sd.FinishedAt != nil:
		res.CompletedAt = sd.FinishedAt.UTC().Format(time.RFC3339)
	case sd.StoppedAt != nil:
		res.CompletedAt = sd.StoppedAt.UTC().Format(time.RFC3339)
	}
}

func failureText(failures []types.Failure) string {
	if len(failures) == 0 {
		return "not found"
	}
	f := failures[0]
	text := aws.ToString(f.Reason)
	if detail := aws.ToString(f.Detail); detail != "" {
		text += ": " + detail
	}
	return text
}
