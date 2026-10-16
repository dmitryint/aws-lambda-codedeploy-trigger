# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Amazon ECS native deployment trigger for services on the `ECS` deployment controller, with any strategy including `BLUE_GREEN`. An event with `cluster`, `service`, `taskDefinition` and optional `forceNewDeployment` calls `UpdateService`, then polls `DescribeServiceDeployments` until the started service deployment is `SUCCESSFUL`, and fails on `ROLLBACK_SUCCESSFUL`, `ROLLBACK_FAILED` or `STOPPED`.
- Result fields `trigger`, `serviceDeploymentArn`, `lifecycleStage`, `statusReason` and `noOp`. Existing fields are unchanged; CodeDeploy results now also carry `trigger: "codedeploy"`.
- Terraform example `examples/terraform-ecs-native-blue-green`.
- Optional `ecs_service_arns` and `ecs_pass_role_arns` grants in `examples/terraform`.

### Changed

- An event that sets both `applicationName` and `service`, or neither, is rejected with a validation error.
- AWS SDK clients are created once per Lambda execution environment instead of on every invocation.

### Fixed

- When waiting, the handler now stops 5 seconds before the Lambda timeout, so the timeout error with the deployment ID and last status reaches the caller instead of a bare `Task timed out`.

## [0.1.0] - 2026-05-09

### Added

- Initial public release.
- AWS Lambda handler that creates a CodeDeploy deployment and waits for completion.
- Synchronous mode (default) and fire-and-forget mode via `wait: false`.
- Configurable poll interval via `POLL_INTERVAL_SECONDS` env var.
- JSON result payload with `deploymentId`, `status`, `createdAt`, `completedAt`.
- Forwarded `CreateDeployment` fields: `description`, `ignoreApplicationStopFailures`, `autoRollbackConfiguration`.
- Build pipeline producing `linux/amd64` and `linux/arm64` `bootstrap` zips for the `provided.al2023` runtime.
- Terraform examples: standalone Lambda provisioning and full ECS Blue/Green flow.
- Unit tests for the event/result schemas.
- GitHub Actions: tests on PR, release artifacts on GitHub Release.

[Unreleased]: https://github.com/dmitryint/aws-lambda-codedeploy-trigger/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/dmitryint/aws-lambda-codedeploy-trigger/releases/tag/v0.1.0
