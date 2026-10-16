variable "lambda_function_name" {
  description = "Name of the codedeploy-trigger Lambda (output of the basic example or your own provisioning)."
  type        = string
}

variable "cluster_arn" {
  description = "ARN of the ECS cluster that runs the service."
  type        = string
}

variable "service_name" {
  description = "Name of the ECS service and task definition family."
  type        = string
  default     = "my-service"
}

variable "desired_count" {
  description = "Number of tasks the service runs."
  type        = number
  default     = 2
}

variable "container_name" {
  description = "Container name in the task definition that the load balancer targets."
  type        = string
  default     = "app"
}

variable "container_image" {
  description = "Container image to run, e.g. 123456789012.dkr.ecr.us-east-1.amazonaws.com/my-service:1.2.3."
  type        = string
}

variable "container_port" {
  description = "Container port that the load balancer targets."
  type        = number
  default     = 8080
}

variable "task_cpu" {
  description = "Fargate task CPU units."
  type        = number
  default     = 256
}

variable "task_memory" {
  description = "Fargate task memory in MiB."
  type        = number
  default     = 512
}

variable "task_execution_role_arn" {
  description = "ARN of the ECS task execution role."
  type        = string
}

variable "subnet_ids" {
  description = "Subnets the tasks run in."
  type        = list(string)
}

variable "security_group_ids" {
  description = "Security groups attached to the tasks."
  type        = list(string)
}

variable "production_target_group_arn" {
  description = "Target group that receives production traffic before the first deployment."
  type        = string
}

variable "alternate_target_group_arn" {
  description = "Target group the green service revision registers with during a deployment."
  type        = string
}

variable "production_listener_rule_arn" {
  description = "ARN of the listener rule that routes production traffic."
  type        = string
}

variable "test_listener_rule_arn" {
  description = "ARN of the listener rule that routes test traffic. Leave null to skip test traffic."
  type        = string
  default     = null
}

variable "load_balancer_role_arn" {
  description = "ARN of the ECS infrastructure role that lets ECS manage the load balancer during deployments."
  type        = string
}

variable "bake_time_in_minutes" {
  description = "Minutes both service revisions keep running after production traffic has shifted."
  type        = number
  default     = 5
}
