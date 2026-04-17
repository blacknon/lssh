provider-connector-telnet Provider
=================================

## About

The `telnet` connector provider describes telnet-based access for resolved targets.

This first implementation is intentionally conservative:

- `shell` is supported
- `exec` is not advertised as a distinct capability
- file transfer, forwarding, and mount-related capabilities are not supported

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-connector-telnet"]

[provider.telnet]
plugin = "provider-connector-telnet"
enabled = true
capabilities = ["connector"]
port = "23"
```

## Notes

- `plugin.describe` reports `["connector"]`.
- `health.check` validates static connector config only.
- `connector.describe` uses `target.config.addr` and `target.config.port`.
- `connector.prepare` currently returns a provider-managed shell plan for `shell`.
