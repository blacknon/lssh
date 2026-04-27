Provider
========

## About

The `provider` directory contains external provider implementations used by `lssh`.
Providers are grouped by capability or implementation style:

- [`inventory`](./inventory/README.md): generate `server` entries from cloud or API inventories
- [`mixed`](./mixed/README.md): multi-capability providers that combine inventory with connector behavior
- [`connector`](./connector/README.md): define or mediate how a resolved `server` can actually be connected
- [`secret`](./secret/README.md): resolve `*_ref` values just before connect

Shared provider-side helper libraries live under `../providerutil/`.
Those packages are reusable support code for provider implementations and are intentionally kept outside the capability-oriented `provider/*` layout.

The bundled providers are maintained as a dedicated Go module rooted at [`go.mod`](./go.mod).
That keeps cloud and connector SDK dependencies out of the main command module while still allowing local development through the repository workspace defined in [`../go.work`](../go.work).
When you refresh vendored dependencies, update the root module and the provider module separately.

Each provider is a standalone executable that communicates with `lssh` over JSON via stdin/stdout.

A single provider implementation may support one capability or multiple capabilities.
For example, one executable may expose only `inventory`, while another may expose both `inventory` and `connector`.

Current maturity in `v0.10.0` is intentionally mixed:

- provider-backed inventory and secret resolution are usable as `beta`
- connector-backed access beyond native SSH is still `experimental`

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

## Two Capability Layers

The word `capabilities` is used in two different layers and they should be kept separate.

### 1. Plugin Capabilities

These describe which provider categories a plugin implements.

Examples:

- `inventory`
- `connector`
- `secret`

These are returned by `plugin.describe`.

### 2. Connector Operation Capabilities

These describe what a resolved target can actually do through a connector.

Examples:

- `shell`
- `exec`
- `exec_pty`
- `upload`
- `download`
- `port_forward_local`
- `port_forward_remote`
- `mount`

These are returned by `connector.describe`.

This separation is important because `connector` alone does not tell the caller whether a target supports interactive shell, command execution, or file transfer.

## Transport-Oriented Connector Design

Some connectors are best treated as transport providers rather than as full end-user feature providers.

The clearest case is an OpenSSH-based connector.

In that model:

- OpenSSH is responsible for the base connection
  - authentication
  - bastion / jump host behavior
  - ProxyJump / ProxyCommand compatibility
  - ControlMaster / session reuse
- Go-side code is responsible for higher-level behavior
  - file transfer
  - sync logic
  - mount-facing file operations
  - command integration with `lscp`, `lsftp`, `lssync`, and `lsshfs`

This keeps the connector thin while still allowing the `lssh` family to present a consistent feature set.

### Recommended Transport Capabilities

For transport-oriented connectors, the connector layer may expose finer-grained capabilities internally.

Examples:

- `shell_transport`
- `exec_transport`
- `sftp_transport`
- `port_forward_transport`

These are not necessarily user-facing command capabilities.
Instead, they are building blocks used by higher-level commands.

Example interpretation:

- `shell_transport`
  - the connector can open an interactive shell transport
- `exec_transport`
  - the connector can execute a command transport
- `sftp_transport`
  - the connector can open an SFTP subsystem stream
- `port_forward_transport`
  - the connector can establish forwarding-compatible transport

This model is especially useful when:

- the connector uses OpenSSH for base connectivity
- `lscp` / `lsftp` / `lssync` should use Go-side SFTP logic instead of shelling out to `scp` or `sftp`
- `lsshfs` should use Go-side file operations rather than delegating to an `sshfs` executable

## Command Capability Requirements

Each `cmd/*` command should decide support based on connector operation capabilities, not only on the presence of the `connector` provider category.

Recommended mapping:

| Command | Required operation capabilities | Notes |
| --- | --- | --- |
| `lssh` | `shell` | interactive login/session |
| `lssh command...` | `exec` | non-interactive command execution |
| `lsshell` | `exec` | parallel shell UI sends commands, but does not require connector-backed interactive shell |
| `lsmux` | `shell` or `exec` | pane shells need `shell`; command panes need `exec` |
| `lssh -P` | `shell` or `exec` | same runtime model as `lsmux` |
| `lscp` | `upload`, `download` | exact direction depends on source/target |
| `lsftp` | `upload`, `download` | interactive file transfer |
| `lssync` | `upload`, `download` | bi-directional sync planning may require both |
| `lsshfs` | `mount` | filesystem-like mount capability |
| `lspipe` | `exec` | remote command execution with local piping |

Notes:

- `lsshell` is intentionally different from `lssh`.
  - `lssh` needs connector-backed interactive shell support
  - `lsshell` can still work with connectors that support only remote command execution
- `lsmux` and `lssh -P` should choose capability by pane mode.
  - shell panes use `shell`
  - command panes use `exec`

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

### Capability Source Of Truth

Plugin capabilities should be owned by the plugin source itself.

The recommended model is:

- the plugin executable declares its supported provider categories via `plugin.describe`
- user config may narrow usage of those categories
- user config must not be treated as authoritative for unsupported categories

In other words:

- plugin source is the source of truth for supported provider categories
- config is allowed to restrict usage
- config should not be allowed to invent unsupported categories

Recommended future core behavior:

- call `plugin.describe`
- compare configured plugin capabilities with runtime-declared plugin capabilities
- reject or warn if config asks for unsupported categories

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
- may expose operation-level capabilities such as `shell`, `exec`, and `upload`

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
  - `plugin.describe`
  - `health.check`
  - `connector.describe`
  - `connector.prepare`
  - selected runtime methods such as `connector.shell`, `connector.exec`, and `connector.dial`

However, the current implementation does not yet expose the full recommended protocol above.

Missing or partial pieces today:

- core-side use of `plugin.describe` and `health.check` is still incomplete and continues to evolve
- `warnings` exist in the protocol shape but are not yet produced consistently across all providers

## Current Plugin Fit And Migration Plan

### Inventory Plugins

Current plugins:

- `provider-mixed-aws-ec2`
- `provider-mixed-azure-compute`
- `provider-mixed-gcp-compute`
- `provider-inventory-proxmox`

Current connector-oriented plugins or families:

- `provider-connector-telnet`
- `provider-connector-winrm`
- `provider-connector-openssh`
- `provider-mixed-aws-ec2`
  - exposes both `inventory` and `connector`
  - covers AWS EC2 inventory plus AWS SSM / EC2 Instance Connect Endpoint connector behavior

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

- connector plugins exist today:
  - `provider-connector-openssh`
  - `provider-connector-telnet`
  - `provider-connector-winrm`
  - `provider-mixed-aws-ec2`
    - `inventory`: AWS EC2 inventory
    - `connector`: `aws-ssm`, `aws-eice`

Recommended migration:

1. Keep expanding `plugin.describe` and `connector.describe` as the compatibility boundary.
2. Continue tightening command-side gating based on connector capabilities.
3. Add operation-specific preparation/runtime methods conservatively as real connector backends mature.

## Recommended Next Protocol Steps

To evolve the current implementation without breaking existing users:

1. Keep `inventory.list` and `secret.get` unchanged.
2. Add `plugin.describe` to all plugins.
3. Implement `health.check` in core and providers.
4. Extend the response envelope with optional `warnings`, `details`, and `id`.
5. Design `connector` methods after the runtime capability discovery path is in place.
