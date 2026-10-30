package main

import (
	"context"
	"testing"
	"time"

	"github.com/dmitryint/aws-lambda-codedeploy-trigger/pkg/deploy"

	"github.com/aws/aws-sdk-go-v2/aws"
	cdtypes "github.com/aws/aws-sdk-go-v2/service/codedeploy/types"
)

const (
	testInterval     = time.Millisecond
	testDeploymentID = "d-ABCDEF123"
)

func codeDeployEvent() deploy.Event {
	return deploy.Event{
		ApplicationName:     aws.String("my-app"),
		DeploymentGroupName: aws.String("my-group"),
		Revision:            &cdtypes.RevisionLocation{RevisionType: cdtypes.RevisionLocationTypeAppSpecContent},
	}
}

func TestHandleRequestSuccess(t *testing.T) {
	t.Parallel()

	fake := &fakeCodeDeploy{
		deploymentID: testDeploymentID,
		statuses:     []cdtypes.DeploymentStatus{cdtypes.DeploymentStatusInProgress, cdtypes.DeploymentStatusSucceeded},
	}
	h := handler{codeDeploy: fake, pollInterval: testInterval}

	res, err := h.handleRequest(context.Background(), codeDeployEvent())
	if err != nil {
		t.Fatalf("handleRequest error: %v", err)
	}
	if res.DeploymentID != testDeploymentID || res.Status != string(cdtypes.DeploymentStatusSucceeded) || res.CreatedAt == "" {
		t.Fatalf("result = %+v", res)
	}
	getCalls, in, _ := fake.calls()
	if getCalls != 2 {
		t.Fatalf("GetDeployment calls = %d, want 2", getCalls)
	}
	if aws.ToString(in.ApplicationName) != "my-app" || aws.ToString(in.DeploymentGroupName) != "my-group" || in.Revision == nil {
		t.Fatalf("CreateDeployment input = %+v", in)
	}
}

func TestHandleRequestDeadlineMargin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wait       *bool
		wantMargin time.Duration
	}{
		{name: "waiting shortens the deadline", wantMargin: deadlineMargin},
		{name: "fire and forget keeps the deadline", wait: aws.Bool(false)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := &fakeCodeDeploy{
				deploymentID: testDeploymentID,
				statuses:     []cdtypes.DeploymentStatus{cdtypes.DeploymentStatusSucceeded},
			}
			h := handler{codeDeploy: fake, pollInterval: testInterval}
			event := codeDeployEvent()
			event.Wait = tc.wait

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			parentDeadline, _ := ctx.Deadline()

			if _, err := h.handleRequest(ctx, event); err != nil {
				t.Fatalf("handleRequest error: %v", err)
			}
			_, _, seen := fake.calls()
			if want := parentDeadline.Add(-tc.wantMargin); !seen.Equal(want) {
				t.Fatalf("CreateDeployment deadline = %v, want %v", seen, want)
			}
		})
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

		if deadline, ok := ctx.Deadline(); !ok || !deadline.Equal(parentDeadline.Add(-deadlineMargin)) {
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
