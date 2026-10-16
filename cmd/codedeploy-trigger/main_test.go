package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dmitryint/aws-lambda-codedeploy-trigger/pkg/deploy"

	"github.com/aws/aws-sdk-go-v2/aws"
	cdtypes "github.com/aws/aws-sdk-go-v2/service/codedeploy/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func codeDeployEvent() deploy.Event {
	return deploy.Event{
		ApplicationName:     aws.String("my-app"),
		DeploymentGroupName: aws.String("my-group"),
		Revision:            &cdtypes.RevisionLocation{RevisionType: cdtypes.RevisionLocationTypeAppSpecContent},
	}
}

func TestHandleRequestValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		event   func() deploy.Event
		wantErr string
	}{
		{
			name:    "neither variant",
			event:   func() deploy.Event { return deploy.Event{} },
			wantErr: "either applicationName (CodeDeploy) or service (ECS) is required",
		},
		{
			name: "both variants",
			event: func() deploy.Event {
				e := codeDeployEvent()
				e.Service = aws.String("my-service")
				return e
			},
			wantErr: "mutually exclusive",
		},
		{
			name: "codedeploy without deployment group",
			event: func() deploy.Event {
				e := codeDeployEvent()
				e.DeploymentGroupName = nil
				return e
			},
			wantErr: "deploymentGroupName is required",
		},
		{
			name: "codedeploy without revision",
			event: func() deploy.Event {
				e := codeDeployEvent()
				e.Revision = nil
				return e
			},
			wantErr: "revision is required",
		},
		{
			name: "ecs without cluster",
			event: func() deploy.Event {
				e := ecsEvent()
				e.Cluster = nil
				return e
			},
			wantErr: "cluster is required",
		},
		{
			name: "ecs without task definition or force",
			event: func() deploy.Event {
				e := ecsEvent()
				e.TaskDefinition = nil
				return e
			},
			wantErr: "taskDefinition is required unless forceNewDeployment is true",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ecsFake := &fakeECS{}
			cdFake := &fakeCodeDeploy{}
			h := handler{codeDeploy: cdFake, ecs: ecsFake, pollInterval: testInterval}

			_, err := h.handleRequest(context.Background(), tc.event())
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
			if ecsFake.describeServicesCalls != 0 || len(ecsFake.updateInputs) != 0 || cdFake.createInput != nil {
				t.Fatal("AWS APIs must not be called for an invalid event")
			}
		})
	}
}

func TestHandleRequestRoutesECSForceWithoutTaskDefinition(t *testing.T) {
	t.Parallel()

	fake := startedDeploymentFake(
		serviceDeployment(newDeploymentArn, types.ServiceDeploymentStatusSuccessful, types.ServiceDeploymentLifecycleStageCleanUp, ""),
	)
	h := handler{ecs: fake, pollInterval: testInterval}
	event := ecsEvent()
	event.TaskDefinition = nil
	event.ForceNewDeployment = aws.Bool(true)

	res, err := h.handleRequest(context.Background(), event)
	if err != nil {
		t.Fatalf("handleRequest error: %v", err)
	}
	if res.Trigger != deploy.TriggerECS || res.Status != "SUCCESSFUL" {
		t.Fatalf("result = %+v", res)
	}
	if in := fake.updateInputs[0]; in.TaskDefinition != nil || !in.ForceNewDeployment {
		t.Fatalf("UpdateService input = %+v", in)
	}
}

func TestHandleRequestCodeDeploy(t *testing.T) {
	t.Parallel()

	fake := &fakeCodeDeploy{
		deploymentID: "d-ABCDEF123",
		statuses:     []cdtypes.DeploymentStatus{cdtypes.DeploymentStatusInProgress, cdtypes.DeploymentStatusSucceeded},
	}
	h := handler{codeDeploy: fake, pollInterval: testInterval}

	res, err := h.handleRequest(context.Background(), codeDeployEvent())
	if err != nil {
		t.Fatalf("handleRequest error: %v", err)
	}
	if res.DeploymentID != "d-ABCDEF123" || res.Status != "Succeeded" || res.Trigger != deploy.TriggerCodeDeploy {
		t.Fatalf("result = %+v", res)
	}
	if res.ServiceDeploymentArn != "" || res.NoOp {
		t.Fatalf("ECS fields set on a CodeDeploy result: %+v", res)
	}
	if aws.ToString(fake.createInput.ApplicationName) != "my-app" || aws.ToString(fake.createInput.DeploymentGroupName) != "my-group" {
		t.Fatalf("CreateDeployment input = %+v", fake.createInput)
	}
}

func TestWithDeadlineMargin(t *testing.T) {
	t.Parallel()

	t.Run("shortens a long deadline", func(t *testing.T) {
		t.Parallel()

		parent, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		parentDeadline, _ := parent.Deadline()

		ctx, cancelChild := withDeadlineMargin(parent)
		defer cancelChild()

		deadline, ok := ctx.Deadline()
		if !ok || !deadline.Equal(parentDeadline.Add(-deadlineMargin)) {
			t.Fatalf("deadline = %v, want %v", deadline, parentDeadline.Add(-deadlineMargin))
		}
	})

	t.Run("keeps a deadline shorter than the margin", func(t *testing.T) {
		t.Parallel()

		parent, cancel := context.WithTimeout(context.Background(), deadlineMargin/2)
		defer cancel()
		parentDeadline, _ := parent.Deadline()

		ctx, cancelChild := withDeadlineMargin(parent)
		defer cancelChild()

		if deadline, _ := ctx.Deadline(); !deadline.Equal(parentDeadline) {
			t.Fatalf("deadline = %v, want %v", deadline, parentDeadline)
		}
	})

	t.Run("no deadline", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := withDeadlineMargin(context.Background())
		defer cancel()

		if _, ok := ctx.Deadline(); ok {
			t.Fatal("unexpected deadline")
		}
	})
}
