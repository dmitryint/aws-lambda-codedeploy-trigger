# Terraform example: provision the Lambda

Provisions the `codedeploy-trigger` Lambda function from a published GitHub release artifact.

## Usage

```hcl
module "codedeploy_trigger" {
  source = "github.com/dmitryint/aws-lambda-codedeploy-trigger//examples/terraform?ref=v0.1.0"

  function_name   = "codedeploy-trigger"
  release_version = "v0.1.0"
  architecture    = "arm64"

  codedeploy_service_role_arns = [
    aws_iam_role.codedeploy.arn,
  ]
}
```

For the ECS trigger, grant the services instead of (or next to) the CodeDeploy roles:

```hcl
module "codedeploy_trigger" {
  source = "github.com/dmitryint/aws-lambda-codedeploy-trigger//examples/terraform"

  release_version = var.release_version

  ecs_service_arns = [
    "arn:aws:ecs:us-east-1:123456789012:service/my-cluster/my-service",
  ]
  ecs_pass_role_arns = [
    "arn:aws:iam::123456789012:role/my-service-task",
    "arn:aws:iam::123456789012:role/my-service-execution",
  ]
}
```

## Inputs

| Name | Description | Default |
|---|---|---|
| `function_name` | Lambda function name | `codedeploy-trigger` |
| `release_version` | Release tag to download (e.g. `v0.1.0`) | — |
| `architecture` | `amd64` or `arm64` | `arm64` |
| `timeout_seconds` | Lambda timeout (max `900`) | `900` |
| `memory_size` | Lambda memory in MB | `128` |
| `poll_interval_seconds` | `POLL_INTERVAL_SECONDS` env var | `15` |
| `codedeploy_service_role_arns` | Service role ARNs the Lambda may `iam:PassRole` | `[]` |
| `ecs_service_arns` | ECS service ARNs the Lambda may describe and update, plus their service deployments | `[]` |
| `ecs_pass_role_arns` | Task and task execution role ARNs the Lambda may `iam:PassRole` to `ecs-tasks.amazonaws.com` | `[]` |

## Outputs

| Name | Description |
|---|---|
| `function_name` | Provisioned Lambda name |
| `function_arn` | Provisioned Lambda ARN |
| `role_arn` | IAM role attached to the Lambda |
