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

The provider should return enough metadata for downstream matching and templating, but should stay focused on discovery rather than connection execution.

## What Inventory Providers Should Do

- Discover targets from an external system.
- Convert those targets into `server`-like records.
- Attach metadata used by:
  - `provider.<name>.match.*`
  - `note_template` and `note_append`
  - template fields such as `addr_template` or `server_name_template`
- Support filtering that is native to the inventory source when it improves performance or clarity.
- Return useful errors when authentication, permissions, or API access fail.

## What Inventory Providers Should Not Do

- Resolve secret references that belong in `secret` providers.
- Implicitly choose a different transport model without making that design explicit.
- Depend on local helper commands unless there is no reasonable API/SDK path.
- Hardcode command-specific assumptions for `lssh`, `lscp`, `lsshfs`, and other callers.

## Output Expectations

An inventory provider should aim to return:

- stable server names
- connectable address information when available
- metadata that helps later matching
- notes or source hints when those improve operator understanding

The most important requirement is that the returned data be predictable enough for users to write durable `match` rules around it.

## Filtering Strategy

Filtering can exist at multiple layers.

- In the provider itself
  - Useful when the upstream API can narrow results efficiently.
- In `provider.<name>.when`
  - Useful when the whole provider should be skipped based on client-side conditions.
- In `provider.<name>.match.*`
  - Useful when the inventory result is valid, but SSH-side fields need per-target adjustment.

As a rule:

- use provider-native filters for inventory size and API efficiency
- use `when` to decide whether the provider should run at all
- use `match` to customize the generated hosts

## Design Notes For New Inventory Providers

When designing a new inventory provider, it helps to answer these questions first:

1. What is the authoritative source of target truth?
2. What target identifiers are stable over time?
3. Which metadata fields are useful for `match` and human-readable notes?
4. Can the address be determined directly, or must it be templated?
5. Which filters belong at the API layer, and which belong in config?
6. What failure modes should be surfaced clearly to users?

Keeping those decisions explicit tends to make the provider easier to configure and easier to debug.
