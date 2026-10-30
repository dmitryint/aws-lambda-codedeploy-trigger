package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dmitryint/aws-lambda-codedeploy-trigger/pkg/deploy"

	"github.com/aws/aws-sdk-go-v2/aws"
	cdtypes "github.com/aws/aws-sdk-go-v2/service/codedeploy/types"
)

func TestRunCodeDeployValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(e *deploy.Event)
		wantErr string
	}{
		{name: "missing application", mutate: func(e *deploy.Event) { e.ApplicationName = nil }, wantErr: "applicationName is required"},
		{name: "empty application", mutate: func(e *deploy.Event) { e.ApplicationName = aws.String("") }, wantErr: "applicationName is required"},
		{name: "missing deployment group", mutate: func(e *deploy.Event) { e.DeploymentGroupName = nil }, wantErr: "deploymentGroupName is required"},
		{name: "missing revision", mutate: func(e *deploy.Event) { e.Revision = nil }, wantErr: "revision is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := &fakeCodeDeploy{deploymentID: testDeploymentID}
			event := codeDeployEvent()
			tc.mutate(&event)

			_, err := runCodeDeploy(context.Background(), fake, event, testInterval)
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
			if getCalls, createInput, _ := fake.calls(); createInput != nil || getCalls != 0 {
				t.Fatal("CodeDeploy must not be called for an invalid event")
			}
		})
	}
}

func TestRunCodeDeployFailedDeployment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  cdtypes.DeploymentStatus
		message string
	}{
		{name: "failed", status: cdtypes.DeploymentStatusFailed, message: "The deployment failed because too many individual instances failed deployment."},
		{name: "stopped", status: cdtypes.DeploymentStatusStopped, message: "The deployment was stopped by a user."},
		{name: "stopped without error information", status: cdtypes.DeploymentStatusStopped},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := &fakeCodeDeploy{
				deploymentID: testDeploymentID,
				statuses:     []cdtypes.DeploymentStatus{cdtypes.DeploymentStatusInProgress, tc.status},
				errorMessage: tc.message,
			}

			res, err := runCodeDeploy(context.Background(), fake, codeDeployEvent(), testInterval)
			if err == nil {
				t.Fatal("runCodeDeploy returned nil error")
			}
			want := "deployment " + testDeploymentID + " ended with status " + string(tc.status)
			if tc.message != "" {
				want += ": " + tc.message
			}
			if err.Error() != want {
				t.Fatalf("error = %q, want %q", err, want)
			}
			if res.DeploymentID != testDeploymentID || res.Status != string(tc.status) {
				t.Fatalf("result = %+v", res)
			}
		})
	}
}

func TestRunCodeDeployContextDeadline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		blockAfterCalls int
	}{
		{name: "between polls"},
		{name: "during GetDeployment", blockAfterCalls: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := &fakeCodeDeploy{
				deploymentID:    testDeploymentID,
				statuses:        []cdtypes.DeploymentStatus{cdtypes.DeploymentStatusInProgress},
				blockAfterCalls: tc.blockAfterCalls,
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()

			res, err := runCodeDeploy(ctx, fake, codeDeployEvent(), testInterval)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error = %v, want context.DeadlineExceeded", err)
			}
			for _, part := range []string{testDeploymentID, "last status: InProgress"} {
				if !strings.Contains(err.Error(), part) {
					t.Fatalf("error %q does not contain %q", err, part)
				}
			}
			if res.DeploymentID != testDeploymentID || res.Status != string(cdtypes.DeploymentStatusInProgress) {
				t.Fatalf("result = %+v", res)
			}
		})
	}
}

func TestRunCodeDeployNoWait(t *testing.T) {
	t.Parallel()

	fake := &fakeCodeDeploy{deploymentID: testDeploymentID}
	event := codeDeployEvent()
	event.Wait = aws.Bool(false)

	res, err := runCodeDeploy(context.Background(), fake, event, testInterval)
	if err != nil {
		t.Fatalf("runCodeDeploy error: %v", err)
	}
	if res.DeploymentID != testDeploymentID || res.Status != string(cdtypes.DeploymentStatusCreated) || res.CreatedAt == "" || res.CompletedAt != "" {
		t.Fatalf("result = %+v", res)
	}
	if getCalls, _, _ := fake.calls(); getCalls != 0 {
		t.Fatalf("GetDeployment calls = %d, want 0", getCalls)
	}
}

func TestRunCodeDeployAPIErrors(t *testing.T) {
	t.Parallel()

	t.Run("create deployment", func(t *testing.T) {
		t.Parallel()

		apiErr := errors.New("DeploymentGroupDoesNotExistException")
		fake := &fakeCodeDeploy{createErr: apiErr}

		res, err := runCodeDeploy(context.Background(), fake, codeDeployEvent(), testInterval)
		if !errors.Is(err, apiErr) || !strings.HasPrefix(err.Error(), "create deployment: ") {
			t.Fatalf("error = %v", err)
		}
		if res != (deploy.Result{}) {
			t.Fatalf("result = %+v, want zero value", res)
		}
		if getCalls, _, _ := fake.calls(); getCalls != 0 {
			t.Fatalf("GetDeployment calls = %d, want 0", getCalls)
		}
	})

	t.Run("get deployment", func(t *testing.T) {
		t.Parallel()

		apiErr := errors.New("ThrottlingException")
		fake := &fakeCodeDeploy{deploymentID: testDeploymentID, getErr: apiErr}

		res, err := runCodeDeploy(context.Background(), fake, codeDeployEvent(), testInterval)
		if !errors.Is(err, apiErr) || !strings.HasPrefix(err.Error(), "get deployment "+testDeploymentID+": ") {
			t.Fatalf("error = %v", err)
		}
		if res.DeploymentID != testDeploymentID || res.Status != string(cdtypes.DeploymentStatusCreated) {
			t.Fatalf("result = %+v", res)
		}
	})
}
