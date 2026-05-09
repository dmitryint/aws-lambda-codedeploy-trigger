# Terraform example: ECS Blue/Green deployment trigger

This example wires the `codedeploy-trigger` Lambda into a Terraform-managed ECS Blue/Green stack.

It assumes the following resources already exist (managed by your own Terraform code or modules):

- An ECS cluster, service, and task definition. The service uses `deployment_controller { type = "CODE_DEPLOY" }`.
- An Application Load Balancer with two target groups (blue/green) and a listener.
- A CodeDeploy application of compute platform `ECS` and a deployment group bound to the service, ALB, and target groups.
- The `codedeploy-trigger` Lambda, deployed via [`../terraform`](../terraform) or your own provisioning.

## Usage

```hcl
module "ecs_blue_green_deploy" {
  source = "github.com/dmitryint/aws-lambda-codedeploy-trigger//examples/terraform-ecs-blue-green?ref=v0.1.0"

  codedeploy_lambda_function_name  = module.codedeploy_trigger.function_name
  codedeploy_application_name      = module.code_deploy.name
  codedeploy_deployment_group_name = module.code_deploy.name

  task_definition_arn = module.ecs_service.task_definition_arn
  container_name      = local.container_name
  container_port      = local.container_port
}

output "last_deployment_id" {
  value = module.ecs_blue_green_deploy.deployment_id
}
```

The `aws_lambda_invocation` re-runs whenever `task_definition_arn` changes, which is the right boundary for an ECS Blue/Green roll-out: a new task definition revision means a new deployment.

## Failure semantics

If the deployment fails or is rolled back, the Lambda returns a non-empty `errorMessage` and `terraform apply` exits non-zero with the message. The CodeDeploy console keeps the full deployment history for diagnostics.

## Why this pattern?

`aws_ecs_service` with `CODE_DEPLOY` controller does not roll out new task definitions on its own — Terraform updates the resource but leaves the live service untouched until a deployment is started. Without this Lambda you have to create the deployment manually (console, CLI, or CodePipeline). With it, `terraform apply` is a single, end-to-end roll-out.
