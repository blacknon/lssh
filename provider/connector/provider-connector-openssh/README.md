provider-connector-openssh Provider
==================================

## About

The `openssh` connector provides OpenSSH-based transport plans for `shell`, `exec`, and `sftp_transport`.

This connector is intentionally transport-oriented:

- `ssh` is used for the base connection
- higher-level file features are expected to be implemented in Go on top of the returned transport plans

That means the long-term intended model is:

- `lssh`
  - uses `shell` / `exec`
- `lscp`, `lsftp`, `lssync`
  - use `sftp_transport`
- `lsshfs`
  - uses `sftp_transport` with Go-side mount integration

At the current stage:

- `lscp`
  - uses `sftp_transport`
- `lsftp`
  - uses `sftp_transport`
- `lssync`
  - uses `sftp_transport`
- `lsshfs`
  - uses `sftp_transport` on Linux and macOS

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-connector-openssh"]

[provider.openssh]
plugin = "provider-connector-openssh"
enabled = true
capabilities = ["connector"]
ssh_path = "/usr/bin/ssh"
strict_host_key_checking = "accept-new"
ssh_options = ["IdentitiesOnly=yes"]

[server.example]
addr = "example.internal"
user = "demo"
port = "2222"
key = "~/.ssh/id_ed25519"
provider_name = "openssh"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- current plugin capabilities are `["connector"]`.
- implemented methods are:
  - `plugin.describe`
  - `health.check`
  - `connector.describe`
  - `connector.prepare`
- `health.check` validates that the configured `ssh` executable can be found.
- supported operation capabilities currently include:
  - `shell`
  - `exec`
  - `exec_pty`
  - `shell_transport`
  - `exec_transport`
  - `sftp_transport`
  - `port_forward_transport`
  - `upload`
  - `download`
  - `port_forward_local`
  - `port_forward_remote`
  - `agent_forward`
- `mount` is advertised on Linux and macOS, where the caller can use the Go-side `sftp_transport` + FUSE backend.
- `connector.prepare` currently returns `plan.kind = "command"` plans that invoke `ssh`.
- the connector itself does not call `scp`, `sftp`, `rsync`, or `sshfs`.
  - those higher-level behaviors are expected to be implemented in Go on top of the returned transport plans
- configurable keys:
  - `ssh_path`
  - `identity_file`
  - `ssh_config_file`
  - `user_known_hosts_file`
  - `strict_host_key_checking`
  - `batch_mode`
  - `forward_agent`
  - `ssh_options`
  - target-side `addr`, `user`, `port`, `key`
