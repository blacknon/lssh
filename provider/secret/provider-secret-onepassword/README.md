provider-secret-onepassword Provider
====================================

## About

The `onepassword` secret provider resolves values with the official 1Password Go SDK.

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-secret-onepassword"]

[provider.onepassword]
plugin = "provider-secret-onepassword"
enabled = true
capabilities = ["secret"]
token = "ops_xxx"

[server.prod]
addr = "10.0.0.10"
user = "ec2-user"
key_ref = "onepassword:op://Infra/prod/key/private"
keypass_ref = "onepassword:op://Infra/prod/key/passphrase"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- Uses the official 1Password Go SDK.
- `token` is required and should be a 1Password service account token.
- Secret refs use the normal 1Password secret reference format such as `op://Vault/item/field`.
