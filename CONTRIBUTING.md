# Contributing

Thanks for taking the time to contribute. This document describes how to set up your environment, the conventions this project follows, and what is expected from a pull request.

## Code of conduct

Be respectful, be constructive. Assume good intent. Harassment of any kind is not tolerated.

## Ways to contribute

- **Report a bug** — open a GitHub issue with reproduction steps, expected vs. actual behavior, the deployment configuration, and Lambda logs (with secrets redacted).
- **Suggest a feature** — open an issue first. Aligning on the JSON wire format before code review keeps the public API stable.
- **Send a pull request** — see [Pull requests](#pull-requests) below.
- **Improve docs** — `README.md`, examples, and inline docs are fair game.

For security issues, do **not** open a public issue. Email the maintainer (see `LICENSE` / git history).

## Prerequisites

- **Go 1.26** — pinned in `go.mod` and CI. Match the local toolchain to avoid `go.mod` churn.
- **Terraform >= 1.5.0** and AWS provider `>= 5.0` if you touch `examples/`.
- `make`, `zip`, and a POSIX shell for the build targets.

## Project layout

| Path | What lives there |
|---|---|
| `cmd/codedeploy-trigger/` | Lambda entry point (`main.go`). |
| `pkg/deploy/` | `Event` (request) and `Result` (response) JSON schemas — **public API**. |
| `examples/terraform/` | Provisions the Lambda from a release artifact. |
| `examples/terraform-ecs-blue-green/` | End-to-end ECS Blue/Green flow. |
| `vendor/` | Committed. Always build and test with `-mod=vendor`. |

## Development workflow

1. Fork the repository and create a topic branch from `main`.
2. Make focused changes — one concern per PR.
3. Run the checks listed below.
4. Open a pull request using the template.

### Build, test, lint

```sh
make test          # go test -mod=vendor -race -count=1 ./...
make vet           # go vet -mod=vendor ./...
make fmt           # go fmt ./...
make tidy          # go mod tidy (only if you touched go.mod)
make package       # builds linux_amd64 and linux_arm64 bootstrap zips
```

For Terraform changes:

```sh
terraform fmt -check -recursive examples/
terraform -chdir=examples/terraform init -backend=false
terraform -chdir=examples/terraform validate
terraform -chdir=examples/terraform-ecs-blue-green init -backend=false
terraform -chdir=examples/terraform-ecs-blue-green validate
```

### Vendoring

Dependencies live in `vendor/` and are committed. If you add or upgrade a module:

```sh
go get example.com/module@vX.Y.Z
go mod tidy
go mod vendor
```

Commit the `vendor/` changes alongside the `go.mod` / `go.sum` updates.

## Coding conventions

- **Wire format is public API.** `pkg/deploy.Event` and `pkg/deploy.Result` JSON shapes are consumed by Terraform users. Adding optional fields is non-breaking; renaming or removing requires a major version bump (or a `0.x` minor pre-1.0). Discuss in an issue first.
- Event JSON keys are lower-camel-case and mirror AWS CodeDeploy `CreateDeployment` (`applicationName`, `deploymentGroupName`, `revision`, …). Do not deviate.
- Errors from `CreateDeployment` / `GetDeployment` are wrapped with the deployment ID so callers can locate the in-flight deployment in the AWS console.
- Polling loops MUST respect `ctx.Done()`. The 15-minute Lambda ceiling surfaces as context cancellation; the surfaced error must include the deployment ID and the last observed status.
- AWS region is read from the standard Lambda environment — do not require explicit `AWS_REGION` / `AWS_DEFAULT_REGION` configuration.
- The handler returns a struct (`pkg/deploy.Result`) — never a plain string.
- The `bootstrap` binary must stay CGO-free and statically linked so it runs on `provided.al2023` without extra dependencies.
- Comments explain **why**, not what. Self-documenting code with good names beats narration.
- Follow standard `gofmt` and the rules in `.golangci.yml`. The CI pipeline runs `golangci-lint`.

## Tests

- Unit tests live next to the code (`*_test.go`).
- Cover both happy path and failure paths — especially around the polling loop, context cancellation, and the JSON wire format.
- Tests must be deterministic and run under `-race`.
- Do not depend on real AWS calls; stub the CodeDeploy client behind an interface.

## Commit messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short summary>

<optional body explaining the why>
```

Common types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`, `build`.

Examples from the repo's history:

- `fix(lint): use US 'canceled' to satisfy misspell linter`
- `fix(ci): bump golangci-lint-action to v9 + golangci-lint v2.12.2 for Go 1.26`

Keep the subject line under ~72 characters. Reference issues with `Fixes #N` or `Refs #N` in the body when relevant.

## Pull requests

A PR is ready for review when:

- [ ] `make test` passes locally with `-race`.
- [ ] `make vet` is clean.
- [ ] `golangci-lint run` is clean (or, equivalently, CI passes).
- [ ] Terraform examples pass `terraform fmt -check -recursive examples/` and `terraform validate`.
- [ ] `CHANGELOG.md` has an entry under `## [Unreleased]` if the change is user-visible.
- [ ] `README.md` / examples are updated when behavior changes.
- [ ] Commits are focused and follow Conventional Commits.

CI runs the test matrix and the release workflow on tagged releases. PRs that change the public JSON schema or the release artifacts need maintainer sign-off before merge.

## Release process

Releases are cut by a maintainer:

1. Move the `## [Unreleased]` items in `CHANGELOG.md` into a new `## [vX.Y.Z] - YYYY-MM-DD` section in a PR; merge to `main`.
2. Create a GitHub Release with tag `vX.Y.Z`.
3. `release.yml` attaches `lambda-codedeploy-trigger_<version>_linux_<arch>.zip` and `.zip.sha256` for both `amd64` and `arm64`.

Pre-1.0 (current): minor bumps may include breaking JSON schema changes; document them prominently in the changelog.

## Hard constraints

- AWS Lambda max runtime is **15 minutes**. Any feature requiring longer waits must use `wait: false` and rely on caller-side polling.
- The `bootstrap` binary must stay CGO-free and statically linked.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
