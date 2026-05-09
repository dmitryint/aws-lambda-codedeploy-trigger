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

## Outputs

| Name | Description |
|---|---|
| `function_name` | Provisioned Lambda name |
| `function_arn` | Provisioned Lambda ARN |
| `role_arn` | IAM role attached to the Lambda |
