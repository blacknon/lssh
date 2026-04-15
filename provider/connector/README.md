Connector Providers
===================

Connector providers are not implemented yet, but this directory is reserved for the provider type that describes how a resolved target can actually be used.

This document uses `connector` as the provider category name.
If older discussion or notes use the spelling `connecter`, they refer to the same design direction.

## Why A Separate Provider Type May Be Needed

Some targets are not well described by ordinary SSH fields alone.

Examples:

- WinRM
- telnet
- serial console
- AWS SSM

These cases often differ by supported operations, not just by host/port/user values.

## Role

A connector provider should answer:

- what operations are supported for this target
- how those operations should be prepared
- what prerequisites or limitations apply

This is different from:

- `inventory`
  - which discovers the target
- `secret`
  - which resolves credentials or tokens

## Relationship With Inventory

A connector provider may depend on metadata produced by inventory.

Examples:

- EC2 inventory returns `instance_id`, `region`, and `platform`
- an AWS SSM connector uses that metadata to decide whether `shell` or `exec` are available
- a serial-console connector may consume `node`, `device`, or `slot` metadata from inventory

This is expected and acceptable.

The intended boundary is:

- `inventory`
  - discovers targets and emits stable metadata
- `connector`
  - consumes target identity and metadata to describe supported operations

## Multi-Capability Plugin Model

The design does not require one executable per provider category.

Both of these are valid:

- separate executables
  - `provider-inventory-aws-ec2`
  - `provider-connector-aws-ssm`
- one executable with multiple capabilities
  - a single provider binary that exposes both `inventory` and `connector`

The important rule is that the runtime contract remains explicit.

## Connector JSON API

Connector providers should use the common provider envelope described in [../README.md](../README.md).

### Recommended Methods

- `plugin.describe`
- `health.check`
- `connector.describe`
- `connector.prepare`

The repository currently has a placeholder constant named `transport.prepare`.
For long-term consistency, the recommended external method naming is `connector.prepare`.

## `connector.describe`

This is the recommended read-only capability discovery method.

Request:

```json
{
  "version": "v1",
  "method": "connector.describe",
  "params": {
    "provider": "aws_ssm",
    "config": {},
    "target": {
      "name": "aws:web-01",
      "config": {
        "addr": "10.0.10.5"
      },
      "meta": {
        "instance_id": "i-0123456789abcdef0",
        "region": "ap-northeast-1",
        "platform": "linux"
      }
    }
  }
}
```

Result:

```json
{
  "capabilities": {
    "shell": {
      "supported": true
    },
    "exec": {
      "supported": true,
      "pty": true
    },
    "upload": {
      "supported": false
    }
  }
}
```

### `target` Fields

- `name`
  - resolved server name
- `config`
  - resolved server config fragment
- `meta`
  - inventory metadata

### Capability Guidance

Recommended capability keys:

- `shell`
- `exec`
- `pty_exec`
- `upload`
- `download`
- `mount`
- `port_forward_local`
- `port_forward_remote`
- `agent_forward`

Each capability object should at minimum expose:

- `supported`

Optional fields may include:

- `pty`
- `reason`
- `constraints`

## `connector.prepare`

This method is intended to prepare a concrete operation after capability discovery.

Request:

```json
{
  "version": "v1",
  "method": "connector.prepare",
  "params": {
    "provider": "aws_ssm",
    "config": {},
    "target": {
      "name": "aws:web-01",
      "meta": {
        "instance_id": "i-0123456789abcdef0",
        "region": "ap-northeast-1"
      }
    },
    "operation": {
      "name": "exec",
      "command": ["uname", "-a"],
      "pty": true
    }
  }
}
```

Result:

```json
{
  "supported": true,
  "plan": {
    "kind": "provider-managed",
    "operation": "exec"
  }
}
```

The exact shape of `plan` is intentionally left open for now.
The main design goal is to keep operation preparation explicit and structured rather than collapsing everything into opaque shell strings.

## Why Capability Separation Matters

Different `lssh` family commands need different things:

- `lssh`
  - interactive shell or equivalent session
- `lscp`, `lsftp`, `lssync`
  - upload and/or download support
- `lsshfs`
  - mount support
- `lspipe`
  - command execution support
- `lsmux`
  - shell or PTY-capable execution

Without a capability model, users may be offered a target that looks available but cannot actually perform the requested action.

## AWS SSM-Like Cases

AWS SSM is a representative example.

A practical design may look like either of these:

- separate providers
  - inventory discovers EC2 instances
  - connector describes SSM-based access for those instances
- one executable with multiple capabilities
  - useful when both features rely on the same upstream API and metadata model

Either shape can work.
The recommendation is:

- keep the config and method contracts separated
- allow one binary to implement multiple capabilities when that reduces duplication
- document the inventory metadata that the connector consumes

## Current Fit And Migration Plan

Current fit:

- no connector plugins exist yet
- no connector methods are implemented in the repository today
- only a placeholder `transport.prepare` constant exists in `internal/providerapi/protocol.go`

Recommended migration:

1. Add `plugin.describe` support to the common provider layer.
2. Introduce `connector.describe` as the first real connector method.
3. Implement command-side gating based on connector capabilities.
4. Add `connector.prepare` only after the capability model is validated by real use cases.
