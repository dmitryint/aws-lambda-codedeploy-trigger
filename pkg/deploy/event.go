package deploy

import (
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/codedeploy/types"
)

// Trigger identifies which deployment system an Event drives.
type Trigger string

const (
	// TriggerCodeDeploy starts an AWS CodeDeploy deployment via CreateDeployment.
	TriggerCodeDeploy Trigger = "codedeploy"
	// TriggerECS starts an Amazon ECS native service deployment via UpdateService.
	TriggerECS Trigger = "ecs"
)

// Event is the JSON payload accepted by the Lambda handler.
//
// The CodeDeploy variant uses the keys of the AWS CodeDeploy CreateDeployment API
// and the ECS variant uses the keys of the Amazon ECS UpdateService API, both in
// lower-camel-case, so a Terraform `aws_lambda_invocation` block can pass arguments
// with the same keys it would use against the AWS API directly.
type Event struct {
	ApplicationName               *string                          `json:"applicationName"`
	DeploymentGroupName           *string                          `json:"deploymentGroupName"`
	Revision                      *types.RevisionLocation          `json:"revision"`
	Description                   *string                          `json:"description,omitempty"`
	IgnoreApplicationStopFailures *bool                            `json:"ignoreApplicationStopFailures,omitempty"`
	AutoRollbackConfiguration     *types.AutoRollbackConfiguration `json:"autoRollbackConfiguration,omitempty"`

	Cluster            *string `json:"cluster,omitempty"`
	Service            *string `json:"service,omitempty"`
	TaskDefinition     *string `json:"taskDefinition,omitempty"`
	ForceNewDeployment *bool   `json:"forceNewDeployment,omitempty"`

	// Wait controls whether the handler blocks until the deployment finishes.
	// Defaults to true; set to false for fire-and-forget invocations.
	Wait *bool `json:"wait,omitempty"`
}

// Trigger reports which deployment system the event targets. Exactly one of
// applicationName (CodeDeploy) and service (ECS) must be set.
func (e Event) Trigger() (Trigger, error) {
	hasApplication := nonEmpty(e.ApplicationName)
	hasService := nonEmpty(e.Service)
	switch {
	case hasApplication && hasService:
		return "", errors.New("applicationName and service are mutually exclusive")
	case hasApplication:
		return TriggerCodeDeploy, nil
	case hasService:
		return TriggerECS, nil
	default:
		return "", errors.New("either applicationName (CodeDeploy) or service (ECS) is required")
	}
}

// ShouldWait reports whether the handler blocks until the deployment finishes.
func (e Event) ShouldWait() bool {
	return e.Wait == nil || *e.Wait
}

func nonEmpty(s *string) bool {
	return s != nil && *s != ""
}

// Result is returned to the caller as the Lambda response payload.
//
// For the ECS trigger, DeploymentID and ServiceDeploymentArn both carry the
// service deployment ARN.
type Result struct {
	DeploymentID string `json:"deploymentId"`
	Status       string `json:"status"`
	CreatedAt    string `json:"createdAt,omitempty"`
	CompletedAt  string `json:"completedAt,omitempty"`

	Trigger              Trigger `json:"trigger,omitempty"`
	ServiceDeploymentArn string  `json:"serviceDeploymentArn,omitempty"`
	LifecycleStage       string  `json:"lifecycleStage,omitempty"`
	StatusReason         string  `json:"statusReason,omitempty"`
	NoOp                 bool    `json:"noOp,omitempty"`
}
