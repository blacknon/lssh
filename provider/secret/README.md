Secret Providers
================

Secret providers resolve `*_ref` values at connection time.

- [`provider-secret-onepassword`](./provider-secret-onepassword/README.md)
- [`provider-secret-bitwarden`](./provider-secret-bitwarden/README.md)
- [`provider-secret-os-keychain`](./provider-secret-os-keychain/README.md)
- [`provider-secret-custom-script`](./provider-secret-custom-script/README.md)

## Role

Secret providers exist to resolve sensitive values as late as possible, ideally just before the value is needed for an actual connection or provider request.

This keeps long-lived config files readable while avoiding direct storage of every secret in plain text.

## Secret JSON API

Secret providers use the common provider envelope described in [../README.md](../README.md).

### Required Methods

- `secret.get`

### Recommended Methods

- `plugin.describe`
- `health.check`

## `secret.get`

Request:

```json
{
  "version": "v1",
  "method": "secret.get",
  "params": {
    "provider": "onepassword",
    "config": {},
    "ref": "op://vault/item/field",
    "server": "prod-web-01",
    "field": "pass"
  }
}
```

Request fields:

- `provider`
- `config`
- `ref`
- `server`
  - optional resolved target name
- `field`
  - optional target config field name, such as `pass` or `token`

Result:

```json
{
  "value": "secret-value",
  "type": "password"
}
```

Result fields:

- `value`
  - resolved secret value
- `type`
  - optional value type hint

Recommended `type` examples:

- `password`
- `token`
- `private_key`
- `passphrase`
- `text`

## Error Guidance

Secret providers should try to distinguish these failure classes cleanly:

- malformed reference
- secret not found
- access denied
- backend unavailable
- authentication failed

These should eventually map to stable `error.code` values in the common response envelope.

## Current Plugin Fit

Current secret plugins already fit the core shape of this API well.

They currently:

- implement `secret.get`
- implement `plugin.describe`
- implement `health.check`
- accept `provider`, `config`, `ref`, `server`, and `field`
- return `value`

They do not yet:

- populate `result.type` consistently
- return structured `error.code`

## Migration Guidance For Existing Plugins

### `provider-secret-onepassword`

Current fit:

- good fit for `secret.get`
- backend API use is aligned with the design direction

Recommended updates:

- add `plugin.describe`
- add `health.check`
- return stable `error.code`
- populate `result.type` when appropriate

### `provider-secret-bitwarden`

Current fit:

- good fit for `secret.get`
- backend CLI use is aligned with the design direction

Recommended updates:

- add `plugin.describe`
- add `health.check`
- return stable `error.code`
- populate `result.type`

### `provider-secret-os-keychain`

Current fit:

- good fit for the external `secret.get` protocol
- backend implementation currently uses the macOS `security` command

Recommended updates:

- add `plugin.describe`
- add `health.check`
- return stable `error.code`

Notes:

- this plugin is outwardly protocol-compliant already
- its internal use of `security` is backend-specific, not a provider-protocol issue

### `provider-secret-custom-script`

Current fit:

- good fit for the external `secret.get` protocol
- intentionally acts as a flexible escape hatch

Recommended updates:

- add `plugin.describe`
- add `health.check`
- document that its child-script protocol is separate from the outer provider protocol

Notes:

- today the child script interface is env/stdout based
- that internal script contract does not need to match the provider JSON protocol unless the project later chooses to standardize it
