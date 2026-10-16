variable "function_name" {
  description = "Name of the Lambda function."
  type        = string
  default     = "codedeploy-trigger"
}

variable "release_version" {
  description = "Release tag to deploy, e.g. v0.1.0."
  type        = string
}

variable "architecture" {
  description = "Lambda architecture. One of: amd64, arm64."
  type        = string
  default     = "arm64"

  validation {
    condition     = contains(["amd64", "arm64"], var.architecture)
    error_message = "architecture must be one of: amd64, arm64."
  }
}

variable "timeout_seconds" {
  description = "Lambda timeout in seconds. Cap is 900 (15 minutes)."
  type        = number
  default     = 900
}

variable "memory_size" {
  description = "Lambda memory size in MB."
  type        = number
  default     = 128
}

variable "poll_interval_seconds" {
  description = "Interval between deployment status polls (CodeDeploy GetDeployment or ECS DescribeServiceDeployments)."
  type        = number
  default     = 15
}

variable "codedeploy_service_role_arns" {
  description = "ARNs of CodeDeploy service roles the Lambda is allowed to PassRole. Leave empty to skip the iam:PassRole grant."
  type        = list(string)
  default     = []
}

variable "ecs_service_arns" {
  description = "ARNs of ECS services the Lambda may update and whose service deployments it may describe. Leave empty to skip the ECS grants."
  type        = list(string)
  default     = []

  validation {
    condition     = alltrue([for arn in var.ecs_service_arns : can(regex("^arn:[^:]+:ecs:[^:]+:[0-9]{12}:service/[^/]+/[^/]+$", arn))])
    error_message = "ecs_service_arns must be ECS service ARNs in the form arn:aws:ecs:<region>:<account>:service/<cluster>/<service>."
  }
}

variable "ecs_pass_role_arns" {
  description = "ARNs of task roles and task execution roles the Lambda may pass to ECS. Leave empty to skip this iam:PassRole grant."
  type        = list(string)
  default     = []
}
