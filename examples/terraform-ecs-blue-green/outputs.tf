output "deployment_id" {
  description = "CodeDeploy deployment ID created by this apply."
  value       = jsondecode(aws_lambda_invocation.deploy.result).deploymentId
}

output "deployment_status" {
  description = "Final CodeDeploy deployment status."
  value       = jsondecode(aws_lambda_invocation.deploy.result).status
}
