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
- [`lsdiff`](./lsdiff/README.md): A synchronized TUI diff viewer for comparing remote files across multiple hosts.
- [`lsshfs`](./lsshfs/README.md): A single-host mount command that uses FUSE on Linux and NFS on macOS. Windows is not supported in `0.10.0`.
- [`lsmon`](./lsmon/README.md): A TUI monitor for viewing the status of multiple hosts side by side.
- [`lspipe`](./lspipe/README.md): A persistent pipe-oriented runner for reusing selected SSH hosts from local shell pipelines. FIFO bridge features are Unix-only.
