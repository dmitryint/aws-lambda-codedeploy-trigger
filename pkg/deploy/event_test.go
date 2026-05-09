package deploy

import (
	"encoding/json"
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
