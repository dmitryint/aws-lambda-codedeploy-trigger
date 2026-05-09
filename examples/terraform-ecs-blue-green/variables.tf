variable "codedeploy_lambda_function_name" {
  description = "Name of the codedeploy-trigger Lambda (output of the basic example or your own provisioning)."
  type        = string
}

variable "codedeploy_application_name" {
  description = "CodeDeploy application name."
  type        = string
}

variable "codedeploy_deployment_group_name" {
  description = "CodeDeploy deployment group name."
  type        = string
}

variable "task_definition_arn" {
  description = "ARN of the ECS task definition revision to roll out."
  type        = string
}

variable "container_name" {
  description = "Container name in the task definition that the load balancer targets."
  type        = string
}

variable "container_port" {
  description = "Container port that the load balancer targets."
  type        = number
}
