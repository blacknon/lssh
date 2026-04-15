Provider
========

## About

The `provider` directory contains external provider implementations used by `lssh`.
Providers are grouped by capability:

- [`inventory`](./inventory/README.md): generate `server` entries from cloud or API inventories
- [`connecter`](./connecter/README.md): define or mediate how a resolved `server` can actually be connected
- [`secret`](./secret/README.md): resolve `*_ref` values just before connect

Each provider is a standalone executable that communicates with `lssh` over JSON via stdin/stdout.

A single provider implementation may support one capability or multiple capabilities.
For example, one executable may expose only `inventory`, while another may expose both `inventory` and `connecter`.

## Design Overview

The provider layer exists to keep `lssh` itself small while allowing environment-specific behavior to be added outside the core commands.

The high-level split is:

- `inventory`
  - Expands dynamic infrastructure into `server` candidates.
  - Examples: cloud instances, Proxmox guests, virtual machines discovered from an API.
- `connecter`
  - Describes how `lssh` should connect to an already resolved server.
  - Intended for cases where the target is not a normal SSH host, or where only part of the command set is available.
- `secret`
  - Resolves credentials or secure values as late as possible, ideally just before use.

These categories are intentionally separate.

- `inventory` answers: "what targets exist?"
- `connecter` answers: "how can this target be used?"
- `secret` answers: "how do we obtain the sensitive values needed to use it?"

Keeping them separate avoids mixing target discovery, auth resolution, and transport behavior into one provider type.

At the same time, the capability split is a design boundary, not necessarily a binary-per-capability rule.
If it improves implementation clarity, one provider executable may implement multiple capability families as long as the runtime contract stays explicit.

## Shared Principles

All provider types should follow these design principles:

- Keep the contract small and explicit.
  - Providers should expose only the minimum data needed for the caller to decide the next step.
- Prefer API or SDK based access over shelling out to local commands.
  - This is especially important for cloud and on-prem inventory/connecter providers.
- Return structured data first.
  - `lssh` should avoid parsing human-oriented output when machine-readable fields are available.
- Be capability-driven.
  - A provider should clearly indicate what it can and cannot do.
- Fail in a debuggable way.
  - Errors should help the user identify whether the problem is credentials, network reachability, permissions, or provider-side limitations.
- Preserve cross-command consistency.
  - Shared provider behavior should work predictably across `lssh`, `lscp`, `lsftp`, `lssync`, `lsshfs`, `lsmux`, and related commands.

## Expected Boundaries

### Inventory Provider Boundary

An inventory provider should focus on discovery and metadata attachment.

It should:

- enumerate candidate targets
- attach metadata that can be consumed by `match`, `when`, templates, and notes
- avoid making assumptions about which command will use the target

It should not:

- decide the final transport behavior for unrelated commands
- fetch secrets that belong in `secret`
- silently embed connector-specific behavior into generic `server` fields unless that behavior is clearly documented

### Connecter Provider Boundary

A connecter provider is intended for cases where "SSH to host:port" is not enough to describe the actual connection model.

Examples:

- API-backed exec/session access
- serial/console style access
- environments where command execution is possible but file transfer is not
- environments where shell login is possible but mount or SFTP is not

Its primary role is to describe capabilities and the connection path, not to rediscover inventory.

In practice, a connecter provider may still depend on metadata originally produced by `inventory`.
That dependency is acceptable as long as:

- the inventory-side metadata contract is explicit
- the connecter-side behavior remains capability-oriented
- the provider does not blur discovery and transport into undocumented side effects

### Secret Provider Boundary

A secret provider should resolve a secret reference into a usable value close to execution time.

It should:

- resolve credentials, tokens, passwords, keys, or related values
- keep secret handling localized
- avoid leaking provider-specific secret semantics into unrelated config handling

It should not:

- enumerate hosts
- decide transport behavior
- act as a general-purpose command runner

## Recommended Evolution

Current provider types already cover discovery and secret resolution well.
If `connecter` is added in the future, the recommended rollout is:

1. Define the capability model first.
2. Add documentation and config shape before runtime behavior.
3. Start with read-only capability discovery.
4. Introduce command-specific execution paths only after the supported/unsupported matrix is clear.

This keeps the implementation grounded in user-visible behavior instead of growing an overly broad abstraction too early.
