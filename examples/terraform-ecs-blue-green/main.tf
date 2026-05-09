terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

# AppSpec content describing the ECS Blue/Green deployment.
# CodeDeploy reads this to swap traffic from the blue target group to the
# green target group running the new task definition.
locals {
  appspec = {
    version = "0.0"
    Resources = [{
      TargetService = {
        Type = "AWS::ECS::Service"
        Properties = {
          TaskDefinition = var.task_definition_arn
          LoadBalancerInfo = {
            ContainerName = var.container_name
            ContainerPort = var.container_port
          }
          PlatformVersion = "LATEST"
        }
      }
    }]
  }
}

# Synchronous invocation — terraform apply blocks until CodeDeploy reports
# Succeeded or Failed. Re-runs only when task_definition_arn changes.
resource "aws_lambda_invocation" "deploy" {
  function_name = var.codedeploy_lambda_function_name

  input = jsonencode({
    applicationName     = var.codedeploy_application_name
    deploymentGroupName = var.codedeploy_deployment_group_name
    description         = "Deployed by Terraform"
    revision = {
      revisionType = "AppSpecContent"
      appSpecContent = {
        content = jsonencode(local.appspec)
        sha256  = sha256(jsonencode(local.appspec))
      }
    }
  })

  triggers = {
    task_definition_arn = var.task_definition_arn
  }
}
