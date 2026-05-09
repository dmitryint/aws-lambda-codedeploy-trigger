output "function_name" {
  description = "Name of the deployed Lambda function."
  value       = aws_lambda_function.this.function_name
}

output "function_arn" {
  description = "ARN of the deployed Lambda function."
  value       = aws_lambda_function.this.arn
}

output "role_arn" {
  description = "ARN of the IAM role attached to the Lambda."
  value       = aws_iam_role.this.arn
}
