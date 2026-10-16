# Terraform example: ECS native blue/green deployment trigger

This example defines an ECS service that uses the Amazon ECS deployment controller with the `BLUE_GREEN` strategy, and rolls out every new task definition revision through the `codedeploy-trigger` Lambda. No CodeDeploy application or deployment group is involved.

It assumes the following resources already exist:

- An ECS cluster, a task execution role, subnets and security groups for Fargate tasks.
- An Application Load Balancer with two target groups (production and alternate), a production listener rule and, optionally, a test listener rule.
- An [ECS infrastructure role for load balancers](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/AmazonECSInfrastructureRolePolicyForLoadBalancers.html) that ECS assumes to shift traffic.
- The `codedeploy-trigger` Lambda with the ECS permissions from the main [README](../../README.md#iam-policy-for-the-lambda-role), deployed via [`../terraform`](../terraform) (set `ecs_service_arns` and `ecs_pass_role_arns`) or your own provisioning.

## Usage

```hcl
module "ecs_native_blue_green" {
  source = "github.com/dmitryint/aws-lambda-codedeploy-trigger//examples/terraform-ecs-native-blue-green"

  lambda_function_name    = module.codedeploy_trigger.function_name
  cluster_arn             = "arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster"
  service_name            = "my-service"
  container_image         = "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-service:1.2.3"
  task_execution_role_arn = "arn:aws:iam::123456789012:role/my-service-execution"

  subnet_ids         = ["subnet-0123456789abcdef0", "subnet-0123456789abcdef1"]
  security_group_ids = ["sg-0123456789abcdef0"]

  production_target_group_arn  = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-service-blue/0123456789abcdef"
  alternate_target_group_arn   = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-service-green/0123456789abcdef"
  production_listener_rule_arn = "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/my-alb/0123456789abcdef/0123456789abcdef/0123456789abcdef"
  load_balancer_role_arn       = "arn:aws:iam::123456789012:role/my-ecs-infrastructure-for-load-balancers"
}
```

The service ignores changes to `task_definition`, so Terraform only registers new task definition revisions. The `aws_lambda_invocation` re-runs whenever the task definition ARN changes and calls `UpdateService` with it, which starts the blue/green service deployment. The Lambda waits until the deployment is `SUCCESSFUL`.

On the first apply, creating the service already starts a deployment of the initial revision. If the invocation that follows starts no new deployment, the Lambda waits for that in-flight one and reports `noOp: true`, so the first apply also ends only when the initial revision is live.

## Failure semantics

If the deployment circuit breaker rolls the deployment back, or the deployment is stopped, the Lambda returns an error naming the service deployment ARN, its status (`ROLLBACK_SUCCESSFUL`, `ROLLBACK_FAILED` or `STOPPED`), the status reason and the last lifecycle stage. `aws_lambda_invocation` turns that into a failed `terraform apply`.

## Fitting into 15 minutes

A Lambda invocation runs for at most 15 minutes, and the handler stops waiting 5 seconds before its timeout. Task start-up, health checks, test and production traffic shifts, any lifecycle hooks and `bake_time_in_minutes` all have to fit into that window. If they cannot, invoke with `wait = false` and watch the service deployment elsewhere.

## Outputs

| Name | Description |
|---|---|
| `service_deployment_arn` | Service deployment the last invocation waited on, `null` when the service has no deployment history |
| `service_deployment_status` | Final service deployment status, e.g. `SUCCESSFUL` |
| `no_op` | `true` when `UpdateService` started no new deployment |
