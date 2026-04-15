provider-secret-onepassword Provider
====================================

## About

The `onepassword` secret provider resolves values either with the official 1Password Go SDK or with the `op` CLI session.

## Example

Service account token:

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-secret-onepassword"]

[provider.onepassword]
plugin = "provider-secret-onepassword"
enabled = true
capabilities = ["secret"]
auth_mode = "service_account"
token = "ops_xxx"

[server.prod]
addr = "10.0.0.10"
user = "ec2-user"
key_ref = "onepassword:op://Infra/prod/key/private"
keypass_ref = "onepassword:op://Infra/prod/key/passphrase"
```

CLI session after `op signin`:

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-secret-onepassword"]

[provider.onepassword]
plugin = "provider-secret-onepassword"
enabled = true
capabilities = ["secret"]
auth_mode = "cli"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- `auth_mode` can be one of:
  - `auto`
  - `service_account`
  - `cli`
- `auth_mode = "auto"` is the default.
- In `auto` mode:
  - if `token` is available, the provider uses the 1Password Go SDK
  - otherwise it falls back to the `op` CLI session
- In `service_account` mode:
  - the provider uses the official 1Password Go SDK
  - `token` is required and should be a 1Password service account token
- In `cli` mode:
  - the provider uses `op read <ref>`
  - `op signin` or Desktop integration must already be available in the local environment
- You can provide the service account token in one of these forms:
  - `token = "ops_xxx"`
  - `token_env = "OP_SERVICE_ACCOUNT_TOKEN"`
  - `token_source = "~/.config/lssh/provider-onepassword.env"`
- When `token_source` is used, the file is parsed as an env file like `KEY=value` or `export KEY=value`.
- `token_source_env` can be used to select the variable name inside the source file. If omitted, `TOKEN` is used.
- `op_path` can be used to override the CLI path. If omitted, `op` is used.
- Secret refs use the normal 1Password secret reference format such as `op://Vault/item/field`.
- For CI or non-interactive environments, `service_account` is recommended.
- For personal interactive environments where `op signin` is already used, `cli` or `auto` is usually the easiest option.
