package deploy

import "github.com/aws/aws-sdk-go-v2/service/codedeploy/types"

// Event is the JSON payload accepted by the Lambda handler.
//
// Field names match the AWS CodeDeploy CreateDeployment API in lower-camel-case
// so a Terraform `aws_lambda_invocation` block can pass arguments with the same
// keys it would use against the AWS API directly.
type Event struct {
	ApplicationName               *string                          `json:"applicationName"`
	DeploymentGroupName           *string                          `json:"deploymentGroupName"`
	Revision                      *types.RevisionLocation          `json:"revision"`
	Description                   *string                          `json:"description,omitempty"`
	IgnoreApplicationStopFailures *bool                            `json:"ignoreApplicationStopFailures,omitempty"`
	AutoRollbackConfiguration     *types.AutoRollbackConfiguration `json:"autoRollbackConfiguration,omitempty"`

	// Wait controls whether the handler blocks until the deployment finishes.
	// Defaults to true; set to false for fire-and-forget invocations.
	Wait *bool `json:"wait,omitempty"`
}

// Result is returned to the caller as the Lambda response payload.
type Result struct {
	DeploymentID string `json:"deploymentId"`
	Status       string `json:"status"`
	CreatedAt    string `json:"createdAt,omitempty"`
	CompletedAt  string `json:"completedAt,omitempty"`
}
