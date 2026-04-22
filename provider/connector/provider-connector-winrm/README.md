provider-connector-winrm Provider
================================

## About

The `winrm` connector provider describes WinRM-based access for resolved targets.

This first implementation focuses on capability discovery and provider-managed plans:

- `exec` is supported
- `shell` is optional and enabled only when `enable_shell = true`
- `exec_pty`, file transfer, forwarding, and mount-related capabilities are not supported yet

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-connector-winrm"]

[provider.winrm]
plugin = "provider-connector-winrm"
enabled = true
capabilities = ["connector"]
transport = "https"
port = "5986"
enable_shell = false

[server.windows01]
addr = "windows.local"
user = "Administrator"
pass = "secret"
connector_name = "winrm"
```

## Notes

- `plugin.describe` reports `["connector"]`.
- connector name is `winrm`.
- `health.check` validates static connector config only.
- `connector.describe` reports `exec` as supported when a target address is available.
- `connector.prepare` currently returns provider-managed plans for `exec` and `shell`.
- `shell` remains disabled unless `enable_shell = true`.
