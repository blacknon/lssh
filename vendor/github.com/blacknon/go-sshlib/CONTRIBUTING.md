# Contributing

Thank you for your interest in contributing to `go-sshlib`.

This repository contains the SSH library used by `lssh`. Small fixes, tests, documentation updates, and focused feature improvements are all welcome.

## Before You Start

- Keep changes as small and focused as possible.
- Avoid unrelated refactors in the same pull request.
- Do not change user-facing behavior, config formats, or CLI-related semantics unless the change is intentional and clearly described.
- Be careful with cross-platform behavior. Linux, macOS, and Windows compatibility matters in this repository.
- Do not modify `vendor/`.
- The `internal/third_party` patches and `go.mod` `replace` directives are intentional. Please do not remove or replace them casually.

## Development

Use the Go version declared in `go.mod`.

Common validation commands:

```sh
go test ./...
```

If you changed a specific area, please run the closest relevant tests first, then run the broader test suite when practical.

## Pull Requests

When opening a pull request, please include:

- A short description of what changed
- Why the change is needed
- Any behavior differences or compatibility considerations
- Test coverage or validation notes

If your change affects user-facing behavior, please update the related documentation such as `README.md`.

## Commit Scope

Good pull requests for this repository usually:

- fix one bug
- add one focused improvement
- add or update tests with the code change
- keep behavior changes explicit

Large architectural changes should be discussed before implementation.

## Reporting Bugs

If you found a bug, please open an issue with:

- your environment
- OS details
- Go version
- a minimal reproduction
- expected behavior
- actual behavior

For security issues, please do not open a public issue. See [SECURITY.md](SECURITY.md).
