terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    null = {
      source  = "hashicorp/null"
      version = ">= 3.2"
    }
  }
}

locals {
  release_url = "https://github.com/dmitryint/aws-lambda-codedeploy-trigger/releases/download/${var.release_version}/lambda-codedeploy-trigger_${trimprefix(var.release_version, "v")}_linux_${var.architecture}.zip"
  zip_path    = "${path.module}/.cache/lambda-codedeploy-trigger_${var.release_version}_${var.architecture}.zip"
}

# Download the release artifact once per (version, architecture) pair so
# Terraform can hash and upload it to Lambda.
resource "null_resource" "download" {
  triggers = {
    url = local.release_url
  }

  provisioner "local-exec" {
    command = "mkdir -p ${dirname(local.zip_path)} && curl -fSL -o ${local.zip_path} ${local.release_url}"
  }
}

data "aws_iam_policy_document" "assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "permissions" {
  statement {
    sid    = "CodeDeployControl"
    effect = "Allow"
    actions = [
      "codedeploy:CreateDeployment",
      "codedeploy:GetDeployment",
      "codedeploy:GetDeploymentConfig",
      "codedeploy:RegisterApplicationRevision",
    ]
    resources = ["*"]
  }

  dynamic "statement" {
    for_each = length(var.codedeploy_service_role_arns) > 0 ? [1] : []
    content {
      sid       = "PassCodeDeployServiceRole"
      effect    = "Allow"
      actions   = ["iam:PassRole"]
      resources = var.codedeploy_service_role_arns
    }
  }
}

resource "aws_iam_role" "this" {
  name               = var.function_name
  assume_role_policy = data.aws_iam_policy_document.assume.json
}

resource "aws_iam_role_policy_attachment" "logs" {
  role       = aws_iam_role.this.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy" "this" {
  name   = "${var.function_name}-codedeploy"
  role   = aws_iam_role.this.id
  policy = data.aws_iam_policy_document.permissions.json
}

resource "aws_lambda_function" "this" {
  function_name    = var.function_name
  role             = aws_iam_role.this.arn
  filename         = local.zip_path
  source_code_hash = filebase64sha256(local.zip_path)
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = [var.architecture == "amd64" ? "x86_64" : "arm64"]
  timeout          = var.timeout_seconds
  memory_size      = var.memory_size

  environment {
    variables = {
      POLL_INTERVAL_SECONDS = tostring(var.poll_interval_seconds)
    }
  }

  depends_on = [
    null_resource.download,
    aws_iam_role_policy.this,
  ]
}
