provider-connector-psrp Provider
================================

## About

The `psrp` connector provides PowerShell Remoting Protocol access by returning
local `pwsh`/`powershell` command plans.

This connector is intentionally different from the older `winrm` connector:

- `psrp`
  - uses local PowerShell remoting commands such as `Enter-PSSession` and `Invoke-Command`
- `winrm`
  - talks to WinRS-style command streams directly from Go

That means the current PSRP implementation favors practical interactive shell
behavior over a pure-Go transport implementation.

The long-term intended model is similar to AWS SSM:

- `command` runtime
  - invoke a local helper or PowerShell-based client
- `library` runtime
  - talk PSRP/WSMan directly from Go

At the moment, only `command` runtime is implemented. `library` runtime is
reserved for the future pure-Go `go-psrplib` direction.

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-connector-psrp"]

[provider.psrp]
plugin = "provider-connector-psrp"
enabled = true
capabilities = ["connector"]
enable_shell = true
shell_runtime = "command"
exec_runtime = "command"
pwsh_path = "/opt/homebrew/bin/pwsh"
authentication = "Basic"
https = true
insecure = true

[server.windows01]
addr = "windows.local"
user = "Administrator"
pass = "secret"
port = "5986"
connector_name = "psrp"
```

## Notes

- `plugin.describe` reports connector name `psrp`.
- implemented methods are:
  - `plugin.describe`
  - `health.check`
  - `connector.describe`
  - `connector.prepare`
- `connector.prepare` returns `plan.kind = "command"` plans that invoke local PowerShell.
- supported operation capabilities currently include:
  - `shell`
  - `exec`
- unsupported capabilities currently include:
  - `exec_pty`
  - `upload`
  - `download`
  - `port_forward_local`
  - `port_forward_remote`
  - `mount`
  - `agent_forward`
- configurable keys:
  - `runtime`
  - `shell_runtime`
  - `exec_runtime`
  - `pwsh_path`
  - `pwsh_options`
  - `authentication`
  - `configuration_name`
  - `operation_timeout_sec`
  - target-side `addr`, `user`, `pass`, `port`, `https`, `insecure`
- on macOS/Linux, local PowerShell WSMan support may require PSWSMan or an equivalent WSMan client installation.
- `library` runtime is intentionally not wired yet; the package structure is being kept extractable so it can later move into a standalone `go-psrplib`.
