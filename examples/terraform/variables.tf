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
  description = "Interval between CodeDeploy GetDeployment polls."
  type        = number
  default     = 15
}

variable "codedeploy_service_role_arns" {
  description = "ARNs of CodeDeploy service roles the Lambda is allowed to PassRole. Leave empty to skip the iam:PassRole grant."
  type        = list(string)
  default     = []
}
