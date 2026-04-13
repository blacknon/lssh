provider-secret-bitwarden Provider
==================================

## About

The `bitwarden` secret provider resolves secrets with the official Bitwarden Go SDK.

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-secret-bitwarden"]

[provider.bitwarden]
plugin = "provider-secret-bitwarden"
enabled = true
capabilities = ["secret"]
api_url = "https://vault.example.local/api"
identity_url = "https://vault.example.local/identity"
token = "..."

[server.db]
addr = "10.0.0.20"
user = "admin"
pass_ref = "bitwarden:item-id/password"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- Uses the official Bitwarden Go SDK.
- `token` is required and should be a Bitwarden Secrets Manager access token.
- Ref format is `bitwarden:<secret-id>/<field>`.
- If `<field>` is omitted, `value` is used. Supported fields are `value`, `password`, `note`, `notes`, and `key`.
- For self-hosted or local-network deployments, set either:
  - `server = "https://vault.example.local"` to derive `/api` and `/identity`
  - or `api_url` / `identity_url` explicitly
