Inventory Providers
===================

Inventory providers generate `server` entries dynamically from external systems.

- [`provider-inventory-aws-ec2`](./provider-inventory-aws-ec2/README.md)
- [`provider-inventory-gcp-compute`](./provider-inventory-gcp-compute/README.md)
- [`provider-inventory-proxmox`](./provider-inventory-proxmox/README.md)

## Role

An inventory provider is responsible for turning an external inventory source into candidate `server` entries that `lssh` can consume.

Typical sources include:

- cloud instance inventories
- hypervisor or virtualization APIs
- CMDB-like systems
- internal infrastructure APIs

## Inventory JSON API

Inventory providers use the common provider envelope described in [../README.md](../README.md).

### Required Methods

- `inventory.list`

### Recommended Methods

- `plugin.describe`
- `health.check`

## `inventory.list`

Request:

```json
{
  "version": "v1",
  "method": "inventory.list",
  "params": {
    "provider": "proxmox",
    "config": {}
  }
}
```

Current required request fields:

- `provider`
- `config`

Recommended future-compatible request fields:

- `context`
  - optional client-side context if inventory evaluation ever needs it
- `cursor`
  - optional pagination token
- `limit`
  - optional max result size

Result:

```json
{
  "servers": [
    {
      "name": "pve:sv-pve01:vm-gitlab-runner1",
      "config": {
        "addr": "vm-gitlab-runner1.blckn",
        "note": "proxmox sv-pve01 vmid=10001"
      },
      "meta": {
        "node": "sv-pve01",
        "vmid": "10001",
        "type": "qemu",
        "status": "running"
      }
    }
  ]
}
```

### `servers[]` Fields

- `name`
  - required stable server name
- `config`
  - optional `server`-compatible config fragment
- `meta`
  - optional string map for matching, templating, and connector decisions

Recommended future-compatible fields:

- `id`
  - stable upstream resource identifier
- `labels`
  - optional string map for user-facing tags
- `connector`
  - optional connector hint only if a future connector contract needs it

## Inventory Metadata Guidance

Inventory metadata is especially important because it may be consumed by:

- `provider.<name>.match.*`
- note templates
- future connector providers

When multiple `provider.<name>.match.*` branches match the same generated host, they are applied:

- first by smaller `priority`
- then by declaration order in the config file

This matches the ordering model used by `server.<name>.match.*`.

Good metadata fields are:

- stable
- string-oriented
- directly useful for matching
- clearly sourced from the upstream system

Examples:

- `instance_id`
- `region`
- `zone`
- `node`
- `vmid`
- `status`
- `os_family`

## Partial Success Behavior

Inventory providers may encounter partial failures.

Examples:

- one VM detail call fails but the rest of inventory is still usable
- one zone or region is unavailable
- optional metadata enrichment fails

Recommended behavior:

- return `result` if the overall inventory is still usable
- attach `warnings` in the response envelope when supported
- write diagnostic details to stderr for current compatibility
- return `error` only when the provider cannot produce a meaningful inventory result

## Current Plugin Fit

Current inventory plugins already fit the core shape of this API well.

They currently:

- implement `inventory.list`
- implement `plugin.describe`
- implement `health.check`
- return `servers[].name`
- return `servers[].config`
- return `servers[].meta`

They do not yet:

- return protocol-level `warnings`

## Migration Guidance For Existing Plugins

### `provider-inventory-aws-ec2`

Current fit:

- good fit for `inventory.list`
- metadata already useful for future connector use

Recommended updates:

- add `plugin.describe`
- add `health.check`
- define stable warning/error codes

### `provider-inventory-gcp-compute`

Current fit:

- good fit for `inventory.list`
- metadata already useful for match rules

Recommended updates:

- add `plugin.describe`
- add `health.check`
- define stable warning/error codes

### `provider-inventory-proxmox`

Current fit:

- good fit for `inventory.list`
- already demonstrates partial enrichment and metadata-driven filtering

Recommended updates:

- add `plugin.describe`
- add `health.check`
- move non-fatal stderr warnings into protocol `warnings` when the core supports them
