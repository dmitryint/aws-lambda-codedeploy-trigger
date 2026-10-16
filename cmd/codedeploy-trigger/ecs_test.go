package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dmitryint/aws-lambda-codedeploy-trigger/pkg/deploy"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

const (
	testInterval      = time.Millisecond
	oldDeploymentArn  = "arn:aws:ecs:us-east-1:123456789012:service-deployment/my-cluster/my-service/old0000000000000000"
	newDeploymentArn  = "arn:aws:ecs:us-east-1:123456789012:service-deployment/my-cluster/my-service/new0000000000000000"
	taskDefinitionArn = "arn:aws:ecs:us-east-1:123456789012:task-definition/my-task:42"
)

func ecsEvent() deploy.Event {
	return deploy.Event{
		Cluster:        aws.String("my-cluster"),
		Service:        aws.String("my-service"),
		TaskDefinition: aws.String(taskDefinitionArn),
	}
}

func service(currentDeploymentArn, primaryDeploymentID string) types.Service {
	s := types.Service{
		ServiceName:          aws.String("my-service"),
		DeploymentController: &types.DeploymentController{Type: types.DeploymentControllerTypeEcs},
	}
	if currentDeploymentArn != "" {
		s.CurrentServiceDeployment = aws.String(currentDeploymentArn)
	}
	if primaryDeploymentID != "" {
		s.Deployments = []types.Deployment{{Id: aws.String(primaryDeploymentID), Status: aws.String(primaryDeploymentStatus)}}
	}
	return s
}

func serviceDeployment(arn string, status types.ServiceDeploymentStatus, stage types.ServiceDeploymentLifecycleStage, reason string) describeDeploymentResponse {
	sd := &types.ServiceDeployment{
		ServiceDeploymentArn: aws.String(arn),
		Status:               status,
		LifecycleStage:       stage,
	}
	if reason != "" {
		sd.StatusReason = aws.String(reason)
	}
	return describeDeploymentResponse{deployment: sd}
}

func startedDeploymentFake(deployments ...describeDeploymentResponse) *fakeECS {
	updated := service(newDeploymentArn, "ecs-svc/2")
	return &fakeECS{
		services:    []types.Service{service(oldDeploymentArn, "ecs-svc/1")},
		updated:     &updated,
		deployments: deployments,
	}
}

func TestRunECSSuccess(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)
	finished := created.Add(7 * time.Minute)
	done := serviceDeployment(newDeploymentArn, types.ServiceDeploymentStatusSuccessful, types.ServiceDeploymentLifecycleStageCleanUp, "")
	done.deployment.CreatedAt = &created
	done.deployment.FinishedAt = &finished

	fake := startedDeploymentFake(
		serviceDeployment(newDeploymentArn, types.ServiceDeploymentStatusPending, "", ""),
		serviceDeployment(newDeploymentArn, types.ServiceDeploymentStatusInProgress, types.ServiceDeploymentLifecycleStageProductionTrafficShift, ""),
		done,
	)
	event := ecsEvent()
	event.ForceNewDeployment = aws.Bool(true)

	res, err := runECS(context.Background(), fake, event, testInterval)
	if err != nil {
		t.Fatalf("runECS error: %v", err)
	}

	want := deploy.Result{
		DeploymentID:         newDeploymentArn,
		Status:               "SUCCESSFUL",
		CreatedAt:            "2026-10-16T10:00:00Z",
		CompletedAt:          "2026-10-16T10:07:00Z",
		Trigger:              deploy.TriggerECS,
		ServiceDeploymentArn: newDeploymentArn,
		LifecycleStage:       "CLEAN_UP",
	}
	if res != want {
		t.Fatalf("result mismatch\n got: %+v\nwant: %+v", res, want)
	}

	if len(fake.updateInputs) != 1 {
		t.Fatalf("UpdateService calls = %d, want 1", len(fake.updateInputs))
	}
	in := fake.updateInputs[0]
	if aws.ToString(in.Cluster) != "my-cluster" || aws.ToString(in.Service) != "my-service" ||
		aws.ToString(in.TaskDefinition) != taskDefinitionArn || !in.ForceNewDeployment {
		t.Fatalf("UpdateService input = %+v", in)
	}
	if fake.describeDeploymentCalls != 3 {
		t.Fatalf("DescribeServiceDeployments calls = %d, want 3", fake.describeDeploymentCalls)
	}
}

func TestRunECSFailedDeployment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status types.ServiceDeploymentStatus
		stage  types.ServiceDeploymentLifecycleStage
		reason string
	}{
		{
			name:   "rollback successful",
			status: types.ServiceDeploymentStatusRollbackSuccessful,
			stage:  types.ServiceDeploymentLifecycleStagePostTestTrafficShift,
			reason: "Service deployment rolled back because the circuit breaker threshold was exceeded.",
		},
		{
			name:   "rollback failed",
			status: types.ServiceDeploymentStatusRollbackFailed,
			stage:  types.ServiceDeploymentLifecycleStageProductionTrafficShift,
			reason: "Rollback failed.",
		},
		{
			name:   "stopped",
			status: types.ServiceDeploymentStatusStopped,
			stage:  types.ServiceDeploymentLifecycleStageScaleUp,
			reason: "Service deployment stopped by a newer deployment.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := startedDeploymentFake(
				serviceDeployment(newDeploymentArn, types.ServiceDeploymentStatusInProgress, types.ServiceDeploymentLifecycleStageScaleUp, ""),
				serviceDeployment(newDeploymentArn, types.ServiceDeploymentStatusRollbackInProgress, tc.stage, tc.reason),
				serviceDeployment(newDeploymentArn, tc.status, tc.stage, tc.reason),
			)

			res, err := runECS(context.Background(), fake, ecsEvent(), testInterval)
			if err == nil {
				t.Fatal("runECS returned nil error")
			}
			for _, part := range []string{newDeploymentArn, string(tc.status), tc.reason, string(tc.stage)} {
				if !strings.Contains(err.Error(), part) {
					t.Fatalf("error %q does not contain %q", err, part)
				}
			}
			if res.Status != string(tc.status) || res.LifecycleStage != string(tc.stage) || res.StatusReason != tc.reason {
				t.Fatalf("result = %+v", res)
			}
			if res.ServiceDeploymentArn != newDeploymentArn || res.NoOp {
				t.Fatalf("result = %+v", res)
			}
		})
	}
}

func TestRunECSNoOp(t *testing.T) {
	t.Parallel()

	unchanged := func(currentArn string) *fakeECS {
		before := service(currentArn, "ecs-svc/1")
		after := service(currentArn, "ecs-svc/1")
		return &fakeECS{services: []types.Service{before}, updated: &after}
	}

	t.Run("current deployment already successful", func(t *testing.T) {
		t.Parallel()

		fake := unchanged(oldDeploymentArn)
		fake.deployments = []describeDeploymentResponse{
			serviceDeployment(oldDeploymentArn, types.ServiceDeploymentStatusSuccessful, types.ServiceDeploymentLifecycleStageCleanUp, ""),
		}

		res, err := runECS(context.Background(), fake, ecsEvent(), testInterval)
		if err != nil {
			t.Fatalf("runECS error: %v", err)
		}
		if !res.NoOp || res.Status != "SUCCESSFUL" || res.ServiceDeploymentArn != oldDeploymentArn || res.Trigger != deploy.TriggerECS {
			t.Fatalf("result = %+v", res)
		}
	})

	t.Run("current deployment still in progress is awaited", func(t *testing.T) {
		t.Parallel()

		fake := unchanged(oldDeploymentArn)
		fake.deployments = []describeDeploymentResponse{
			serviceDeployment(oldDeploymentArn, types.ServiceDeploymentStatusInProgress, types.ServiceDeploymentLifecycleStageBakeTime, ""),
			serviceDeployment(oldDeploymentArn, types.ServiceDeploymentStatusSuccessful, types.ServiceDeploymentLifecycleStageCleanUp, ""),
		}

		res, err := runECS(context.Background(), fake, ecsEvent(), testInterval)
		if err != nil {
			t.Fatalf("runECS error: %v", err)
		}
		if !res.NoOp || res.Status != "SUCCESSFUL" || fake.describeDeploymentCalls != 2 {
			t.Fatalf("result = %+v, describe calls = %d", res, fake.describeDeploymentCalls)
		}
	})

	t.Run("current deployment rolled back", func(t *testing.T) {
		t.Parallel()

		fake := unchanged(oldDeploymentArn)
		fake.deployments = []describeDeploymentResponse{
			serviceDeployment(oldDeploymentArn, types.ServiceDeploymentStatusRollbackSuccessful, types.ServiceDeploymentLifecycleStageScaleUp, "Tasks failed to start."),
		}

		res, err := runECS(context.Background(), fake, ecsEvent(), testInterval)
		if err == nil {
			t.Fatal("runECS returned nil error")
		}
		for _, part := range []string{"forceNewDeployment", oldDeploymentArn, "ROLLBACK_SUCCESSFUL"} {
			if !strings.Contains(err.Error(), part) {
				t.Fatalf("error %q does not contain %q", err, part)
			}
		}
		if !res.NoOp {
			t.Fatalf("result = %+v", res)
		}
	})

	t.Run("timeout while current deployment is in progress", func(t *testing.T) {
		t.Parallel()

		fake := unchanged(oldDeploymentArn)
		fake.deployments = []describeDeploymentResponse{
			serviceDeployment(oldDeploymentArn, types.ServiceDeploymentStatusInProgress, types.ServiceDeploymentLifecycleStageBakeTime, ""),
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err := runECS(ctx, fake, ecsEvent(), testInterval)
		if !errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "forceNewDeployment") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("update response without service", func(t *testing.T) {
		t.Parallel()

		fake := unchanged(oldDeploymentArn)
		fake.updated = nil
		fake.deployments = []describeDeploymentResponse{
			serviceDeployment(oldDeploymentArn, types.ServiceDeploymentStatusSuccessful, types.ServiceDeploymentLifecycleStageCleanUp, ""),
		}

		res, err := runECS(context.Background(), fake, ecsEvent(), testInterval)
		if err != nil {
			t.Fatalf("runECS error: %v", err)
		}
		if !res.NoOp || fake.describeServicesCalls != 2 {
			t.Fatalf("result = %+v, DescribeServices calls = %d", res, fake.describeServicesCalls)
		}
	})

	t.Run("no service deployment history", func(t *testing.T) {
		t.Parallel()

		fake := unchanged("")

		res, err := runECS(context.Background(), fake, ecsEvent(), testInterval)
		if err != nil {
			t.Fatalf("runECS error: %v", err)
		}
		if !res.NoOp || res.ServiceDeploymentArn != "" || fake.describeDeploymentCalls != 0 {
			t.Fatalf("result = %+v, describe calls = %d", res, fake.describeDeploymentCalls)
		}
	})
}

func TestRunECSContextTimeout(t *testing.T) {
	t.Parallel()

	fake := startedDeploymentFake(
		serviceDeployment(newDeploymentArn, types.ServiceDeploymentStatusInProgress, types.ServiceDeploymentLifecycleStageBakeTime, ""),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	res, err := runECS(ctx, fake, ecsEvent(), testInterval)
	if err == nil {
		t.Fatal("runECS returned nil error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error %v is not context.DeadlineExceeded", err)
	}
	for _, part := range []string{newDeploymentArn, "last status: IN_PROGRESS", "last lifecycle stage: BAKE_TIME"} {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("error %q does not contain %q", err, part)
		}
	}
	if res.Status != "IN_PROGRESS" || res.LifecycleStage != "BAKE_TIME" || res.ServiceDeploymentArn != newDeploymentArn {
		t.Fatalf("result = %+v", res)
	}
}

func TestRunECSNoWait(t *testing.T) {
	t.Parallel()

	fake := startedDeploymentFake()
	event := ecsEvent()
	event.Wait = aws.Bool(false)

	res, err := runECS(context.Background(), fake, event, testInterval)
	if err != nil {
		t.Fatalf("runECS error: %v", err)
	}
	if res.Status != "PENDING" || res.ServiceDeploymentArn != newDeploymentArn || res.DeploymentID != newDeploymentArn || res.NoOp {
		t.Fatalf("result = %+v", res)
	}
	if fake.describeDeploymentCalls != 0 {
		t.Fatalf("DescribeServiceDeployments calls = %d, want 0", fake.describeDeploymentCalls)
	}
}

func TestRunECSDeploymentArnReportedLate(t *testing.T) {
	t.Parallel()

	updated := service(oldDeploymentArn, "ecs-svc/2")
	fake := &fakeECS{
		services: []types.Service{
			service(oldDeploymentArn, "ecs-svc/1"),
			service(oldDeploymentArn, "ecs-svc/2"),
			service(newDeploymentArn, "ecs-svc/2"),
		},
		updated: &updated,
		deployments: []describeDeploymentResponse{
			serviceDeployment(newDeploymentArn, types.ServiceDeploymentStatusSuccessful, types.ServiceDeploymentLifecycleStageCleanUp, ""),
		},
	}

	res, err := runECS(context.Background(), fake, ecsEvent(), testInterval)
	if err != nil {
		t.Fatalf("runECS error: %v", err)
	}
	if res.NoOp || res.ServiceDeploymentArn != newDeploymentArn || res.Status != "SUCCESSFUL" {
		t.Fatalf("result = %+v", res)
	}
	if fake.describeServicesCalls != 3 {
		t.Fatalf("DescribeServices calls = %d, want 3", fake.describeServicesCalls)
	}
}

func TestRunECSDeploymentNotVisibleYet(t *testing.T) {
	t.Parallel()

	missing := describeDeploymentResponse{failures: []types.Failure{{
		Arn:    aws.String(newDeploymentArn),
		Reason: aws.String("MISSING"),
	}}}

	t.Run("becomes visible", func(t *testing.T) {
		t.Parallel()

		fake := startedDeploymentFake(
			missing,
			serviceDeployment(newDeploymentArn, types.ServiceDeploymentStatusSuccessful, types.ServiceDeploymentLifecycleStageCleanUp, ""),
		)

		res, err := runECS(context.Background(), fake, ecsEvent(), testInterval)
		if err != nil {
			t.Fatalf("runECS error: %v", err)
		}
		if res.Status != "SUCCESSFUL" {
			t.Fatalf("result = %+v", res)
		}
	})

	t.Run("never visible", func(t *testing.T) {
		t.Parallel()

		fake := startedDeploymentFake(missing)

		_, err := runECS(context.Background(), fake, ecsEvent(), testInterval)
		if err == nil || !strings.Contains(err.Error(), "MISSING") || !strings.Contains(err.Error(), newDeploymentArn) {
			t.Fatalf("error = %v", err)
		}
		if fake.describeDeploymentCalls != maxDescribeMisses {
			t.Fatalf("DescribeServiceDeployments calls = %d, want %d", fake.describeDeploymentCalls, maxDescribeMisses)
		}
	})
}

func TestRunECSRejectsOtherDeploymentControllers(t *testing.T) {
	t.Parallel()

	s := service(oldDeploymentArn, "ecs-svc/1")
	s.DeploymentController = &types.DeploymentController{Type: types.DeploymentControllerTypeCodeDeploy}
	fake := &fakeECS{services: []types.Service{s}}

	_, err := runECS(context.Background(), fake, ecsEvent(), testInterval)
	if err == nil || !strings.Contains(err.Error(), "CODE_DEPLOY") {
		t.Fatalf("error = %v", err)
	}
	if len(fake.updateInputs) != 0 {
		t.Fatalf("UpdateService calls = %d, want 0", len(fake.updateInputs))
	}
}

func TestRunECSServiceNotFound(t *testing.T) {
	t.Parallel()

	fake := &fakeECS{serviceFailure: []types.Failure{{Reason: aws.String("MISSING")}}}

	_, err := runECS(context.Background(), fake, ecsEvent(), testInterval)
	if err == nil || !strings.Contains(err.Error(), "MISSING") || !strings.Contains(err.Error(), "my-service") {
		t.Fatalf("error = %v", err)
	}
	if len(fake.updateInputs) != 0 {
		t.Fatalf("UpdateService calls = %d, want 0", len(fake.updateInputs))
	}
}
