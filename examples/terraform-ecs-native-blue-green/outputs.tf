output "service_deployment_arn" {
  description = "ARN of the ECS service deployment the last invocation waited on, null when the service has no deployment history."
  value       = try(jsondecode(aws_lambda_invocation.deploy.result).serviceDeploymentArn, null)
}

output "service_deployment_status" {
  description = "Final ECS service deployment status."
  value       = jsondecode(aws_lambda_invocation.deploy.result).status
}

output "no_op" {
  description = "True when UpdateService started no new deployment."
  value       = try(jsondecode(aws_lambda_invocation.deploy.result).noOp, false)
}
