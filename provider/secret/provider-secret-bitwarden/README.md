provider-secret-bitwarden Provider
==================================

## About

The `bitwarden` secret provider resolves values with the `bw` CLI session.

## Example

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
  - `cli`
- `auth_mode = "auto"` is the default.
- `auth_mode = "auto"` currently behaves the same as `cli` for backward compatibility.
- `auth_mode = "sdk"` is no longer supported.
- The provider uses the `bw` command.
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
- The provider is intended for Bitwarden Password Manager item refs.
  - Supported fields are `value`, `password`, `username`, `uri`, `totp`, `note`, `notes`, `key`, `public_key`, `fingerprint`, and `field:<name>`.
  - `value` is treated the same as `password`.
  - `key` returns `sshKey.privateKey` first, then top-level `key`, then custom field `key`, and finally `notes`.
  - `public_key` returns `sshKey.publicKey`.
  - `fingerprint` returns `sshKey.keyFingerprint`.
  - `field:<name>` returns a custom field value from the item, for example `bitwarden:item-id/field:ssh_key`.
