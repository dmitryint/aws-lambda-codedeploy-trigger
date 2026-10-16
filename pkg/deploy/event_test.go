package deploy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/codedeploy/types"
)

func TestEventUnmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, e Event)
	}{
		{
			name: "appspec content revision",
			input: `{
				"applicationName": "my-app",
				"deploymentGroupName": "my-group",
				"revision": {
					"revisionType": "AppSpecContent",
					"appSpecContent": {
						"content": "version: 0.0",
						"sha256": "abc123"
					}
				}
			}`,
			check: func(t *testing.T, e Event) {
				if got := strDeref(e.ApplicationName); got != "my-app" {
					t.Fatalf("ApplicationName = %q, want %q", got, "my-app")
				}
				if got := strDeref(e.DeploymentGroupName); got != "my-group" {
					t.Fatalf("DeploymentGroupName = %q, want %q", got, "my-group")
				}
				if e.Revision == nil {
					t.Fatal("Revision is nil")
				}
				if e.Revision.RevisionType != types.RevisionLocationTypeAppSpecContent {
					t.Fatalf("RevisionType = %q, want AppSpecContent", e.Revision.RevisionType)
				}
				if e.Revision.AppSpecContent == nil {
					t.Fatal("AppSpecContent is nil")
				}
				if got := strDeref(e.Revision.AppSpecContent.Content); got != "version: 0.0" {
					t.Fatalf("Content = %q", got)
				}
			},
		},
		{
			name: "s3 revision with optional fields",
			input: `{
				"applicationName": "my-app",
				"deploymentGroupName": "my-group",
				"description": "manual rollback",
				"wait": false,
				"ignoreApplicationStopFailures": true,
				"revision": {
					"revisionType": "S3",
					"s3Location": {
						"bucket": "my-bucket",
						"key": "app.zip",
						"bundleType": "zip"
					}
				}
			}`,
			check: func(t *testing.T, e Event) {
				if got := strDeref(e.Description); got != "manual rollback" {
					t.Fatalf("Description = %q", got)
				}
				if e.Wait == nil || *e.Wait != false {
					t.Fatalf("Wait = %v, want false pointer", e.Wait)
				}
				if e.IgnoreApplicationStopFailures == nil || !*e.IgnoreApplicationStopFailures {
					t.Fatal("IgnoreApplicationStopFailures must be true")
				}
				if e.Revision == nil || e.Revision.S3Location == nil {
					t.Fatal("S3Location is nil")
				}
				if got := strDeref(e.Revision.S3Location.Bucket); got != "my-bucket" {
					t.Fatalf("Bucket = %q", got)
				}
			},
		},
		{
			name:    "empty payload",
			input:   `{}`,
			wantErr: false,
			check: func(t *testing.T, e Event) {
				if e.ApplicationName != nil {
					t.Fatal("ApplicationName must be nil")
				}
				if e.Wait != nil {
					t.Fatal("Wait must be nil when omitted")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var e Event
			err := json.Unmarshal([]byte(tc.input), &e)
			if (err != nil) != tc.wantErr {
				t.Fatalf("unmarshal error = %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && tc.check != nil {
				tc.check(t, e)
			}
		})
	}
}

func TestResultMarshal(t *testing.T) {
	t.Parallel()

	r := Result{
		DeploymentID: "d-XXXX",
		Status:       "Succeeded",
		CreatedAt:    "2026-05-09T10:00:00Z",
		CompletedAt:  "2026-05-09T10:05:00Z",
	}

	got, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	want := `{"deploymentId":"d-XXXX","status":"Succeeded","createdAt":"2026-05-09T10:00:00Z","completedAt":"2026-05-09T10:05:00Z"}`
	if string(got) != want {
		t.Fatalf("marshal output mismatch\n got: %s\nwant: %s", got, want)
	}
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func TestEventUnmarshalECS(t *testing.T) {
	t.Parallel()

	var e Event
	input := `{
		"cluster": "my-cluster",
		"service": "my-service",
		"taskDefinition": "arn:aws:ecs:us-east-1:123456789012:task-definition/my-task:42",
		"forceNewDeployment": true,
		"wait": false
	}`
	if err := json.Unmarshal([]byte(input), &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if strDeref(e.Cluster) != "my-cluster" || strDeref(e.Service) != "my-service" {
		t.Fatalf("Cluster = %q, Service = %q", strDeref(e.Cluster), strDeref(e.Service))
	}
	if strDeref(e.TaskDefinition) != "arn:aws:ecs:us-east-1:123456789012:task-definition/my-task:42" {
		t.Fatalf("TaskDefinition = %q", strDeref(e.TaskDefinition))
	}
	if e.ForceNewDeployment == nil || !*e.ForceNewDeployment {
		t.Fatal("ForceNewDeployment must be true")
	}
	if e.ShouldWait() {
		t.Fatal("ShouldWait must be false")
	}
}

func TestEventTrigger(t *testing.T) {
	t.Parallel()

	name := func(s string) *string { return &s }

	tests := []struct {
		name    string
		event   Event
		want    Trigger
		wantErr string
	}{
		{name: "codedeploy", event: Event{ApplicationName: name("my-app")}, want: TriggerCodeDeploy},
		{name: "ecs", event: Event{Service: name("my-service")}, want: TriggerECS},
		{name: "both", event: Event{ApplicationName: name("my-app"), Service: name("my-service")}, wantErr: "mutually exclusive"},
		{name: "neither", event: Event{}, wantErr: "is required"},
		{name: "empty strings count as absent", event: Event{ApplicationName: name(""), Service: name("")}, wantErr: "is required"},
		{name: "empty service with application", event: Event{ApplicationName: name("my-app"), Service: name("")}, want: TriggerCodeDeploy},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.event.Trigger()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Trigger error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Trigger = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEventShouldWaitDefault(t *testing.T) {
	t.Parallel()

	if !(Event{}).ShouldWait() {
		t.Fatal("ShouldWait must default to true")
	}
}

func TestResultMarshalECS(t *testing.T) {
	t.Parallel()

	arn := "arn:aws:ecs:us-east-1:123456789012:service-deployment/my-cluster/my-service/abc123"
	r := Result{
		DeploymentID:         arn,
		Status:               "ROLLBACK_SUCCESSFUL",
		CreatedAt:            "2026-10-16T10:00:00Z",
		CompletedAt:          "2026-10-16T10:09:00Z",
		Trigger:              TriggerECS,
		ServiceDeploymentArn: arn,
		LifecycleStage:       "PRODUCTION_TRAFFIC_SHIFT",
		StatusReason:         "rolled back",
		NoOp:                 true,
	}

	got, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	want := `{"deploymentId":"` + arn + `","status":"ROLLBACK_SUCCESSFUL","createdAt":"2026-10-16T10:00:00Z","completedAt":"2026-10-16T10:09:00Z",` +
		`"trigger":"ecs","serviceDeploymentArn":"` + arn + `","lifecycleStage":"PRODUCTION_TRAFFIC_SHIFT","statusReason":"rolled back","noOp":true}`
	if string(got) != want {
		t.Fatalf("marshal output mismatch\n got: %s\nwant: %s", got, want)
	}
}
