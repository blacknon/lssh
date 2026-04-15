Provider
========

## About

The `provider` directory contains external provider implementations used by `lssh`.
Providers are grouped by capability:

- [`inventory`](./inventory/README.md): generate `server` entries from cloud or API inventories
- [`connector`](./connector/README.md): define or mediate how a resolved `server` can actually be connected
- [`secret`](./secret/README.md): resolve `*_ref` values just before connect

Each provider is a standalone executable that communicates with `lssh` over JSON via stdin/stdout.

A single provider implementation may support one capability or multiple capabilities.
For example, one executable may expose only `inventory`, while another may expose both `inventory` and `connector`.

## Design Goals

The provider protocol should be:

- extensible
  - new methods and optional fields can be added without breaking older plugins
- integrated
  - `inventory`, `connector`, and `secret` use one common JSON envelope
- capability-oriented
  - each plugin can declare what it supports at runtime
- debuggable
  - errors should be machine-readable and helpful to users
- backward-compatible
  - current `inventory.list` and `secret.get` plugins should be migratable with small changes

## Unified JSON Protocol

### Transport

- `lssh` sends exactly one JSON request to provider stdin
- the provider writes exactly one JSON response to stdout
- human-oriented logs should go to stderr
- the provider process exit code should still indicate success or failure
  - but stdout should contain a JSON response even on provider-reported errors when possible

### Common Request Envelope

```json
{
  "version": "v1",
  "id": "optional-request-id",
  "method": "inventory.list",
  "params": {}
}
```

Fields:

- `version`
  - protocol version string
- `id`
  - optional request id for tracing and future multiplexing
- `method`
  - provider method name
- `params`
  - method-specific object

### Common Response Envelope

```json
{
  "version": "v1",
  "id": "optional-request-id",
  "result": {},
  "error": null,
  "warnings": []
}
```

Fields:

- `version`
  - protocol version string
- `id`
  - optional echo of request id
- `result`
  - method-specific result object
- `error`
  - machine-readable error object
- `warnings`
  - optional non-fatal warnings

Exactly one of `result` or `error` should be set.

### Common Error Object

```json
{
  "code": "auth_failed",
  "message": "token is invalid",
  "details": {
    "provider": "proxmox"
  },
  "retryable": false
}
```

Recommended fields:

- `code`
  - stable machine-readable error code
- `message`
  - human-readable summary
- `details`
  - optional method-specific structured details
- `retryable`
  - optional hint for retry behavior

### Common Warning Object

```json
{
  "code": "partial_data",
  "message": "guest ostype could not be fetched for qemu/10082"
}
```

Warnings are optional and should be used for partial success cases where returning `error` would be too strong.

## Common Methods

### `plugin.describe`

This method is the recommended runtime entry point for capability discovery.

Request:

```json
{
  "version": "v1",
  "method": "plugin.describe",
  "params": {}
}
```

Result:

```json
{
  "name": "provider-inventory-proxmox",
  "capabilities": ["inventory"],
  "methods": ["plugin.describe", "health.check", "inventory.list"],
  "protocol_version": "v1"
}
```

Recommended result fields:

- `name`
- `capabilities`
  - one or more of `inventory`, `connector`, `secret`
- `methods`
  - supported method names
- `protocol_version`
- `plugin_version`
  - optional plugin build/version string

### `health.check`

This method is recommended for preflight checks and diagnostics.

Request:

```json
{
  "version": "v1",
  "method": "health.check",
  "params": {
    "provider": "proxmox",
    "config": {}
  }
}
```

Result:

```json
{
  "ok": true,
  "message": "configuration looks valid"
}
```

Recommended result fields:

- `ok`
- `message`
- `checks`
  - optional list of individual check results

## Capability Boundaries

### Inventory

- discovers candidate targets
- returns stable names, config fragments, and metadata
- may be consumed later by `connector`

### Connector

- describes how a resolved target can actually be used
- may depend on metadata produced by `inventory`
- must not silently reimplement inventory discovery as an undocumented side effect

### Secret

- resolves secret references close to execution time
- should not discover targets or define transport behavior

## Compatibility Notes

The current repository already has a minimal shared provider protocol in code:

- request envelope with `version`, `method`, `params`
- response envelope with `version`, `result`, `error`
- implemented methods:
  - `inventory.list`
  - `secret.get`

However, the current implementation does not yet expose the full recommended protocol above.

Missing or partial pieces today:

- no implemented `connector` methods
- no core-side use of `plugin.describe`
- no core-side use of `health.check`
- `warnings` exist in the protocol shape but are not yet produced by current inventory/secret plugins

## Current Plugin Fit And Migration Plan

### Inventory Plugins

Current plugins:

- `provider-inventory-aws-ec2`
- `provider-inventory-gcp-compute`
- `provider-inventory-proxmox`

Current fit:

- already aligned with the common JSON envelope
- already aligned with `inventory.list`
- already return `servers[].name`, `servers[].config`, `servers[].meta`
- partially aligned with the future design because metadata is already exposed

Gaps:

- no structured warning return
  - warnings currently go to stderr in the Proxmox plugin
- no explicit result pagination or cursor support

Recommended migration:

1. Add `plugin.describe` to each inventory plugin.
2. Add `health.check` with cheap auth/config validation where possible.
3. Keep `inventory.list` as-is for backward compatibility.
4. Add optional `warnings` support for partial-success cases.
5. Add optional pagination only if a provider needs it later.

### Secret Plugins

Current plugins:

- `provider-secret-onepassword`
- `provider-secret-bitwarden`
- `provider-secret-os-keychain`
- `provider-secret-custom-script`

Current fit:

- already aligned with the common JSON envelope
- already aligned with `secret.get`
- already accept provider config plus a reference string

Gaps:

- `SecretGetResult.type` is mostly unused
- provider-specific error codes are now partially structured, but not yet normalized across all backends

Recommended migration:

1. Add `plugin.describe` to each secret plugin.
2. Add `health.check` where backend login/config can be validated safely.
3. Start populating `error.code` for stable failure classes.
4. Populate `result.type` when the backend can identify a value type.

Special note:

- `provider-secret-os-keychain`
  - currently shells out to the macOS `security` command
  - outwardly it still follows the provider JSON contract
  - if stricter backend abstraction is needed later, the internal implementation can be revisited without changing the outer protocol
- `provider-secret-custom-script`
  - already follows the provider JSON contract as an `lssh` plugin
  - internally it delegates to an external command through env vars and stdout
  - this is acceptable as an explicit escape hatch, but it should remain documented as a special-case backend

### Connector Plugins

Current fit:

- no connector plugins exist yet

Recommended migration:

1. Implement `plugin.describe` first.
2. Add a read-only connector method for capability discovery.
3. Add operation-specific preparation methods after the capability model is stable.

## Recommended Next Protocol Steps

To evolve the current implementation without breaking existing users:

1. Keep `inventory.list` and `secret.get` unchanged.
2. Add `plugin.describe` to all plugins.
3. Implement `health.check` in core and providers.
4. Extend the response envelope with optional `warnings`, `details`, and `id`.
5. Design `connector` methods after the runtime capability discovery path is in place.
