# Contributing

Thanks for your interest in contributing to `lssh`.

This document explains how to work on the repository, how to validate changes, and what kinds of updates should also include documentation or compatibility checks.

## Before You Start

- Read the main [README.md](./README.md) for the project overview.
- Check the command-specific README under `cmd/*/README.md` when changing user-facing behavior.
- If you touch providers, also read the docs under [provider/](./provider/README.md).

## Development Environment

`lssh` is a Go project with multiple commands and provider binaries.

- Use the Go version declared in [go.mod](./go.mod).
- `mise` tasks are provided for common workflows.
- `make` targets are also available for broader validation.

Typical setup:

```bash
mise run go_mod
```

If you prefer working directly with Go commands, make sure `go mod tidy` and `go mod vendor` are kept in sync through the existing project workflow.

## Repository Layout

High-level structure:

- `cmd/*`
  - CLI entrypoints for each command
- `internal/app/*`
  - command-level application logic
- `internal/*`
  - shared implementation used across commands
- `provider/*`
  - external provider executables and provider design docs
- `docs/*`
  - user documentation
- `completion/*`
  - shell completion files

Main commands in the suite:

- `lssh`
- `lscp`
- `lsftp`
- `lssync`
- `lsdiff`
- `lsshfs`
- `lsmon`
- `lsshell`
- `lsmux`
- `lspipe`

When changing shared code, please consider effects on the whole suite, not just one command.

## Build And Test

Please run tests that are close to your change first.

Examples:

```bash
go test ./internal/app/lssh/...
go test ./internal/lsshfs ./internal/app/lsshfs
go test ./provider/secret/provider-secret-onepassword
```

`mise` tasks are available for grouped workflows:

```bash
mise run core_test
mise run transfer_test
mise run provider_test
```

For broader validation:

```bash
make test
make build
```

If you do not run a test or build step, mention that clearly in your PR description.

## Coding Guidelines

Please keep changes focused and easy to review.

- Keep the scope as local as possible.
- Do not mix unrelated refactors into a behavioral change.
- Preserve existing CLI flags, config formats, and user-facing behavior unless a change is intentional.
- Prefer simple Go implementations over over-engineered abstractions.
- Introduce interfaces only when they are actually needed.
- Return actionable errors rather than swallowing them.
- Add short comments only when they genuinely clarify a non-obvious part of the code.

## Cross-Platform Expectations

This project targets Linux, macOS, and Windows, but some features are OS-specific.

- Call out OS-specific behavior in code when needed.
- Do not let Linux/macOS-specific behavior leak into unrelated commands.
- Be especially careful around `lsshfs`, terminal handling, PTY behavior, and mux/session features.
- If a feature is intentionally unsupported on a platform, document that clearly.

## Provider Development

Providers are treated as separately buildable executables.

- Keep provider-specific helper code under the provider's own directory when possible.
- Do not place provider-only helper libraries in generic shared directories unless they are truly reused across provider types.
- Prefer API or SDK based implementations for cloud providers.
- Avoid local shell command execution inside providers unless there is no practical alternative and the behavior is intentional.
- Keep provider capabilities accurate. User config may limit capabilities, but should not invent unsupported ones.

Useful paths:

- [provider/README.md](./provider/README.md)
- [provider/connector/README.md](./provider/connector/README.md)
- [provider/inventory/README.md](./provider/inventory/README.md)
- [provider/secret/README.md](./provider/secret/README.md)

## Dependency And Vendor Rules

- Do not edit `vendor/` by hand.
- If a problem appears to come from vendored code, first check whether updating the dependency version is the right fix.
- Keep module and vendor state aligned through the project's existing workflow.
- Add new dependencies only when clearly necessary.

## Documentation Updates

If you change any user-facing behavior, also update the related documentation.

This usually means checking:

- [README.md](./README.md)
- `docs/*.md`
- `cmd/*/README.md`
- provider README files when provider behavior changes

Examples of changes that require docs updates:

- new CLI flags
- changed config keys or defaults
- install flow changes
- new platform limitations
- provider capability or protocol changes

## Pull Requests

A good pull request should make review easy.

Please include:

- what changed
- why it changed
- what tests you ran
- what you did not validate
- any OS-specific notes or limitations

If the change is intentionally breaking or changes existing behavior, explain the migration or compatibility impact clearly.

## Reporting Bugs

Bug reports are much easier to act on when they include concrete details.

Please include as much of the following as possible:

- OS and version
- command you ran
- expected behavior
- actual behavior
- relevant config snippet
- provider setup details if applicable
- logs or error output
- reproduction steps

## Design Notes

Some parts of the project have stronger compatibility expectations than others.

- Shared SSH/config behavior should remain stable unless there is a strong reason to change it.
- Provider protocols should evolve carefully and stay documented.
- Mux, transfer, and auth-related changes should be reviewed for side effects on parallel workflows, forwarding, and secret handling.

## Questions

If you are unsure whether a change should be treated as a bug fix, a feature, or a larger architectural change, open an issue or draft PR first and describe the intended direction before making a large change.
