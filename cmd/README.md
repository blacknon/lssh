Cmd
===

## About

The `cmd` directory contains the CLI entry points for the `lssh` suite.
Each command has its own `main.go` and delegates the actual application logic to the implementation under `internal/app/...`.

## Commands

- [`lssh`](./lssh/README.md): A TUI-based SSH client with host selection, parallel command execution, and multiple port-forwarding modes.
- [`lsshell`](./lsshell/README.md): An interactive shell for sending commands to multiple hosts at the same time.
- [`lsftp`](./lsftp/README.md): An interactive SFTP shell for working with one or more hosts from a single interface.
- [`lscp`](./lscp/README.md): A file transfer client that provides an SCP-style interface.
- [`lssync`](./lssync/README.md): A one-way sync command over SSH/SFTP with optional destination pruning.
- [`lsmon`](./lsmon/README.md): A TUI monitor for viewing the status of multiple hosts side by side.

<!-- Note:
    以下のコマンドは後々作っていく

    - lsmux ... v0.8.0
    - lsshfs ... v0.8.0
 -->
