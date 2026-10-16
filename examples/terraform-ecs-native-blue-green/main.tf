terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 6.6.0"
    }
  }
}

resource "aws_ecs_task_definition" "this" {
  family                   = var.service_name
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.task_cpu
  memory                   = var.task_memory
  execution_role_arn       = var.task_execution_role_arn

  container_definitions = jsonencode([{
    name      = var.container_name
    image     = var.container_image
    essential = true
    portMappings = [{
      containerPort = var.container_port
      protocol      = "tcp"
    }]
  }])
}

resource "aws_ecs_service" "this" {
  name            = var.service_name
  cluster         = var.cluster_arn
  task_definition = aws_ecs_task_definition.this.arn
  desired_count   = var.desired_count
  launch_type     = "FARGATE"

  deployment_controller {
    type = "ECS"
  }

  deployment_configuration {
    strategy             = "BLUE_GREEN"
    bake_time_in_minutes = tostring(var.bake_time_in_minutes)
  }

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  load_balancer {
    target_group_arn = var.production_target_group_arn
    container_name   = var.container_name
    container_port   = var.container_port

    advanced_configuration {
      alternate_target_group_arn = var.alternate_target_group_arn
      production_listener_rule   = var.production_listener_rule_arn
      test_listener_rule         = var.test_listener_rule_arn
      role_arn                   = var.load_balancer_role_arn
    }
  }

  network_configuration {
    subnets          = var.subnet_ids
    security_groups  = var.security_group_ids
    assign_public_ip = false
  }

  lifecycle {
    ignore_changes = [task_definition]
  }
}

resource "aws_lambda_invocation" "deploy" {
  function_name = var.lambda_function_name

  input = jsonencode({
    cluster        = var.cluster_arn
    service        = aws_ecs_service.this.name
    taskDefinition = aws_ecs_task_definition.this.arn
  })

  triggers = {
    task_definition_arn = aws_ecs_task_definition.this.arn
  }
}
