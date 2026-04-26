provider-secret-os-keychain Provider
====================================

## About

The `os-keychain` secret provider resolves secrets from the macOS Keychain with the `security` command.

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-secret-os-keychain"]

[provider.keychain]
plugin = "provider-secret-os-keychain"
enabled = true
capabilities = ["secret"]

[server.mac]
addr = "10.0.0.30"
user = "demo"
pass_ref = "keychain:my-service/my-account"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- Requires macOS and the `security` command.
- Ref format is `keychain:<service>/<account>`.
