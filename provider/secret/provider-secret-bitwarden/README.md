provider-secret-bitwarden Provider
==================================

## About

The `bitwarden` secret provider resolves values either with the official Bitwarden Go SDK or with the `bw` CLI session.

## Example

SDK access token:

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-secret-bitwarden"]

[provider.bitwarden]
plugin = "provider-secret-bitwarden"
enabled = true
capabilities = ["secret"]
auth_mode = "sdk"
token_env = "BW_ACCESS_TOKEN"

[server.db]
addr = "10.0.0.20"
user = "admin"
pass_ref = "bitwarden:secret-id/password"
```

CLI session after `bw login` and `bw unlock`:

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-secret-bitwarden"]

[provider.bitwarden]
plugin = "provider-secret-bitwarden"
enabled = true
capabilities = ["secret"]
auth_mode = "cli"

[server.db]
addr = "10.0.0.20"
user = "admin"
pass_ref = "bitwarden:item-id/password"
```

CLI session with an explicit session token:

```toml
[provider.bitwarden]
plugin = "provider-secret-bitwarden"
enabled = true
capabilities = ["secret"]
auth_mode = "cli"
session_env = "BW_SESSION"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- `auth_mode` can be one of:
  - `auto`
  - `sdk`
  - `cli`
- `auth_mode = "auto"` is the default.
  - if `token` is set, the provider uses the SDK
  - otherwise it falls back to the `bw` CLI session
- SDK mode uses the official Bitwarden Go SDK.
- `token` is required for `auth_mode = "sdk"` and should be a Bitwarden Secrets Manager access token.
- You can provide `token` in one of these forms:
  - `token = "..."`
  - `token_env = "BW_ACCESS_TOKEN"`
  - `token_source = "~/.config/lssh/provider-bitwarden.env"`
- When `token_source` is used, the file is parsed as an env file like `KEY=value` or `export KEY=value`.
- `token_source_env` can be used to select the variable name inside the source file. If omitted, `TOKEN` is used.
- CLI mode uses the `bw` command.
  - Log in with `bw login`
  - unlock with `bw unlock`
  - or provide `session`, `session_env`, or `session_source`
- `bw_path` can be used to override the CLI path. If omitted, `bw` is used.
- `appdata_dir` can be used to set `BITWARDENCLI_APPDATA_DIR` for the CLI.
  - This is useful if you want the provider to use a specific Bitwarden CLI data directory.
  - `appdata_dir_env` and `appdata_dir_source` are also supported through the common config value resolver.
- Ref format is `bitwarden:<locator>/<field>`.
  - The provider uses the last `/` as the field separator.
  - This means names or path-like locators such as `bitwarden:folder/item/key` can be used naturally.
- SDK mode is intended for Bitwarden Secrets Manager refs.
  - Supported fields are `value`, `password`, `note`, `notes`, and `key`.
- CLI mode is intended for Bitwarden Password Manager item refs.
  - Supported fields are `value`, `password`, `username`, `uri`, `totp`, `note`, `notes`, `key`, `public_key`, `fingerprint`, and `field:<name>`.
  - In CLI mode, `value` is treated the same as `password`.
  - In CLI mode, `key` returns `sshKey.privateKey` first, then top-level `key`, then custom field `key`, and finally `notes`.
  - In CLI mode, `public_key` returns `sshKey.publicKey`.
  - In CLI mode, `fingerprint` returns `sshKey.keyFingerprint`.
  - `field:<name>` returns a custom field value from the item, for example `bitwarden:item-id/field:ssh_key`.
- For self-hosted or local-network Secrets Manager deployments, set either:
  - `server = "https://vault.example.local"` to derive `/api` and `/identity`
  - or `api_url` / `identity_url` explicitly
