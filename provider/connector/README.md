Connector Providers
===================

Connector providers are not implemented yet, but this directory is reserved for the provider type that describes how a resolved target can actually be used.

This document uses `connector` as the provider category name.
If older discussion or notes use the spelling `connecter`, they refer to the same design direction.

Current prototype providers:

- [`provider-connector-openssh`](./provider-connector-openssh/README.md)
- [`provider-connector-telnet`](./provider-connector-telnet/README.md)
- [`provider-connector-winrm`](./provider-connector-winrm/README.md)

Planned design-only connector families:

- `provider-connector-openssh`
- cloud-specific connectors that may internally use OpenSSH-compatible transport

## Why A Separate Provider Type May Be Needed

Some targets are not well described by ordinary SSH fields alone.

Examples:

- WinRM
- telnet
- serial console
- AWS SSM

These cases often differ by supported operations, not just by host/port/user values.

The first intended connector families are:

- telnet
- winrm
- serial console
- aws ssm
- openssh transport

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
  - `provider-mixed-aws-ec2`
  - `provider-connector-aws-ssm`
- one executable with multiple capabilities
  - a single provider binary that exposes both `inventory` and `connector`

The important rule is that the runtime contract remains explicit.

### Planned AWS EC2 + SSM Shape

AWS SSM is the strongest current case for a multi-capability plugin.

Planned direction:

- current source today
  - `provider/mixed/provider-mixed-aws-ec2`
- current plugin name
  - `provider-mixed-aws-ec2`
- planned source location after connector support
  - keep using `provider/mixed/provider-mixed-aws-ec2`
- planned plugin capabilities
  - `["inventory", "connector"]`

Why a neutral location is preferred:

- the plugin is no longer inventory-only
- placing it only under `inventory/` or only under `connector/` would hide part of its role
- AWS EC2 inventory and AWS SSM connector share one upstream identity model and metadata model

Today the repository layout has already moved to the mixed provider location, while the implemented methods are still inventory-only.

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
- `exec_pty`
- `upload`
- `download`
- `mount`
- `port_forward_local`
- `port_forward_remote`
- `agent_forward`

Transport-oriented connectors may also expose internal transport capabilities:

- `shell_transport`
- `exec_transport`
- `sftp_transport`
- `port_forward_transport`

Recommended interpretation:

- `shell`
  - interactive login-like session
- `exec`
  - non-interactive command execution
- `exec_pty`
  - command execution with PTY allocation
- `upload`
  - local-to-remote file copy
- `download`
  - remote-to-local file copy
- `mount`
  - filesystem mount-like behavior

This layer is different from plugin capabilities.

- plugin capability `connector`
  - means the plugin implements connector methods
- connector operation capability `shell`
  - means the resolved target supports shell access through that connector

Each capability object should at minimum expose:

- `supported`

Optional fields may include:

- `pty`
- `reason`
- `constraints`
- `preferred`
- `requires`

Recommended optional fields for the first implementation wave:

- `reason`
  - short human-readable explanation when unsupported
- `requires`
  - prerequisites such as `["aws:ssm_agent"]` or `["winrm:https"]`
- `constraints`
  - structured limits such as max upload size or unsupported shell flavor
- `preferred`
  - whether this connector is the preferred path when multiple connectors are possible

### Capability Source Of Truth

Connector operation capabilities should be owned by the plugin source and returned by `connector.describe`.

Recommended model:

- plugin source declares what the target supports
- user config may restrict how `lssh` uses those operations
- user config must not create unsupported operations

Examples:

- a user may disable `upload` usage even if the connector supports it
- a user must not be able to declare `mount` support for a WinRM connector that does not implement it

## Current Command Mapping

The current command-to-capability expectation is:

| Command | Capability selection |
| --- | --- |
| `lssh` | `shell` |
| `lssh command...` | `exec` |
| `lsshell` | `exec` |
| `lsmux` | `shell` for shell panes, `exec` for command panes |
| `lssh -P` | same as `lsmux` |
| `lscp` / `lsftp` / `lssync` | `upload` and/or `download` |
| `lsshfs` | `mount` |
| `lspipe` | `exec` |

This means a connector can already be useful even if it does not implement interactive shell.
For example:

- WinRM may still serve `lsshell` and `lspipe` through `exec`
- AWS SSM may serve `lsshell`, command panes, and plain `lssh`
- Telnet is useful mainly for `lssh` and shell panes

## Operation Model

The first connector design should stay conservative.

Recommended first-class operations are:

- `shell`
- `exec`
- `exec_pty`

Recommended second-phase operations are:

- `upload`
- `download`
- `port_forward_local`
- `port_forward_remote`

Recommended later-phase operations are:

- `mount`
- `agent_forward`

This ordering is intentional.
For telnet, WinRM, and AWS SSM, the highest-confidence shared abstraction is session and command execution.
File transfer support differs significantly and should not be over-generalized too early.

For OpenSSH-oriented connectors, however, file-oriented transports are first-class because OpenSSH can provide the underlying connection while Go-side code implements higher-level file behavior.

## Connector-Specific Design

### SSH Reference Model

SSH is not a connector plugin target for the first wave, but it is the baseline comparison model.

Expected operation capabilities:

- `shell`
  - supported
- `exec`
  - supported
- `exec_pty`
  - supported
- `upload`
  - supported
- `download`
  - supported
- `port_forward_local`
  - supported
- `port_forward_remote`
  - supported
- `mount`
  - conditionally supported through `sshfs`

### OpenSSH Connector Model

An OpenSSH connector is different from a plain shell-out wrapper around `scp`, `sftp`, or `sshfs`.

Recommended design:

- use `ssh` only for the base transport
- let Go-side code speak higher-level protocols on top of that transport when practical

This means:

- `lssh`
  - uses OpenSSH for `shell` and `exec`
- `lscp`
  - does not call `scp`
  - uses OpenSSH to establish an SFTP subsystem stream and performs upload/download in Go
- `lsftp`
  - does not call `sftp`
  - uses the same SFTP subsystem transport but keeps the interactive UI in Go
- `lssync`
  - does not call `rsync`
  - reuses Go-side SFTP transfer and tree-walk logic on top of the same transport
- `lsshfs`
  - does not require an `sshfs` executable
  - uses Go-side file operations on top of the same SFTP transport

In other words, the connector provides transport, and the commands provide protocol-aware behavior.

#### Why This Model Is Preferred

This model keeps the OpenSSH connector aligned with the project architecture:

- OpenSSH handles difficult enterprise SSH compatibility
  - ProxyJump
  - bastion flows
  - ControlMaster
  - OpenSSH config compatibility
- Go-side commands keep consistent behavior across connectors
  - file browsing
  - sync planning
  - mount behavior
  - error shaping

It also avoids creating a connector that is merely a thin command launcher for many separate OS tools.

#### Recommended OpenSSH Connector Capabilities

Plugin capability:

- `connector`

Primary operation capabilities:

- `shell`
- `exec`
- `exec_pty`

Primary transport capabilities:

- `shell_transport`
- `exec_transport`
- `sftp_transport`
- `port_forward_transport`

Conditional operation capabilities:

- `upload`
- `download`
- `mount`
- `port_forward_local`
- `port_forward_remote`
- `agent_forward`

Recommended interpretation:

- `upload` / `download`
  - supported when `sftp_transport` is available and the command layer can use it
- `mount`
  - supported when the project's mount layer can operate over the SFTP transport
- forwarding capabilities
  - supported only if the chosen OpenSSH transport plan exposes them safely

#### Go-Side Wrapper Requirement

To make this work cleanly, the project should provide a Go wrapper that is conceptually similar to `go-sshlib`, but backed by OpenSSH processes.

Recommended responsibilities of this wrapper:

- open interactive shell sessions via OpenSSH
- open exec sessions via OpenSSH
- open an SFTP subsystem stream via OpenSSH
- expose stdio-based transport handles that higher-level Go code can use
- expose forwarding-related transport setup where applicable

Recommended non-goals:

- it should not try to reimplement OpenSSH itself
- it should not require `scp`, `sftp`, `rsync`, or `sshfs` executables when Go-side protocol handling already exists

This is best thought of as a `go-sshlib`-compatible OpenSSH transport layer.

#### Example Capability Mapping For OpenSSH Connector

| Command | Required OpenSSH-backed capability | Notes |
| --- | --- | --- |
| `lssh` | `shell` | interactive shell via `ssh` |
| `lssh command...` | `exec` | remote command via `ssh` |
| `lsshell` | `exec` | command execution transport is sufficient |
| `lsmux` shell panes | `shell` | interactive pane transport |
| `lsmux` command panes | `exec` | command pane transport |
| `lscp` | `sftp_transport` | upload/download handled in Go |
| `lsftp` | `sftp_transport` | UI in Go, transport via OpenSSH |
| `lssync` | `sftp_transport` | sync logic in Go |
| `lsshfs` | `sftp_transport` plus mount integration | mount semantics remain Go-side |
| `lspipe` | `exec` | process-oriented command execution |

### Secret Interaction

For cloud and enterprise environments, connectors often depend on `secret` providers even when the connector itself focuses only on transport.

Examples:

- OpenSSH connector
  - `key_ref`
  - `keypass_ref`
  - bastion credentials
- WinRM connector
  - `username_ref`
  - `password_ref`
  - certificate material
- AWS / GCP / Azure connectors
  - API credentials
  - session tokens
  - bastion-side SSH keys

So the expected architecture is often:

- `inventory`
  - target discovery
- `connector`
  - transport and operation planning
- `secret`
  - credential resolution

This means it is correct to think about connector design and secret design together, especially for cloud connectors.

### Cloud Connector Direction

Recommended direction by cloud:

- AWS
  - `provider-mixed-aws-ec2`
    - `inventory`
    - `connector`
  - connector modes may later include:
    - `ssm`
    - `ssh`
    - `bastion_ssh`
- GCP
  - `provider-inventory-gcp-compute`
  - later `provider-connector-gcp-ssh`
    - direct SSH
    - OS Login SSH
    - IAP-backed SSH transport
- Azure
  - `provider-inventory-azure-compute`
  - later connector candidates:
    - `provider-connector-azure-run-command`
    - `provider-connector-azure-ssh`
    - `provider-connector-azure-winrm`
    - `provider-connector-azure-bastion-ssh`

For Bastion-like cloud features:

- generic bastion or jump-host behavior can often remain a proxy concern
- cloud-managed bastion features with cloud-specific setup and capability limits are usually better represented as connectors
- `agent_forward`
  - supported

Why it matters:

- SSH is the richest transport in the current `lssh` family
- other connectors should be compared against this baseline rather than pretending they are equivalent

### Telnet Connector

Planned plugin shape:

- plugin name
  - `provider-connector-telnet`
- plugin capabilities
  - `["connector"]`

Typical target inputs:

- `target.config.addr`
- `target.config.port`
- `target.config.user`
- optional target or provider config for prompt patterns and login sequencing

Expected operation capabilities:

- `shell`
  - supported
- `exec`
  - unsupported as a distinct non-interactive primitive in the first design pass
- `exec_pty`
  - unsupported as a distinct capability
- `upload`
  - unsupported
- `download`
  - unsupported
- `port_forward_local`
  - unsupported
- `port_forward_remote`
  - unsupported
- `mount`
  - unsupported
- `agent_forward`
  - unsupported

Important design notes:

- telnet is line-oriented and session-oriented
- command execution may still be emulated by sending input inside a shell session, but that should not be advertised as true `exec`
- the connector should prefer honesty over convenience

Recommended `connector.prepare` shape:

- `shell`
  - `plan.kind = "command"`
  - local command plan launches a telnet session with structured arguments

### WinRM Connector

Planned plugin shape:

- plugin name
  - `provider-connector-winrm`
- plugin capabilities
  - `["connector"]`

Typical target inputs:

- `target.config.addr`
- `target.config.port`
- `target.config.user`
- credentials from `secret` providers
- transport metadata such as HTTP vs HTTPS

Expected operation capabilities:

- `shell`
  - conditionally supported
  - only if the implementation chooses to provide an interactive PowerShell-like session
- `exec`
  - supported
- `exec_pty`
  - unsupported in the first design pass
- `upload`
  - conditionally supported in a later phase
- `download`
  - conditionally supported in a later phase
- `port_forward_local`
  - unsupported
- `port_forward_remote`
  - unsupported
- `mount`
  - unsupported
- `agent_forward`
  - unsupported

Important design notes:

- WinRM's strongest fit is remote command execution
- interactive shell support is possible but should be treated as distinct from SSH-quality TTY behavior
- file transfer should not be claimed unless there is a deliberate native implementation path

Recommended `connector.prepare` shape:

- `exec`
  - `plan.kind = "provider-managed"` or `plan.kind = "command"`
- `shell`
  - only if an explicit interactive session model is implemented

### AWS SSM Connector

Planned plugin shape:

- plugin name
  - `provider-mixed-aws-ec2`
- plugin capabilities
  - `["inventory", "connector"]`

Typical target inputs:

- inventory metadata from EC2
  - `instance_id`
  - `region`
  - `platform`
  - `availability_zone`
- optional provider config
  - AWS profile
  - shared config files
  - region overrides

Expected operation capabilities:

- `shell`
  - supported for SSM-managed instances
- `exec`
  - supported
- `exec_pty`
  - supported when Session Manager interactive execution is available
- `upload`
  - unsupported in the first design pass
- `download`
  - unsupported in the first design pass
- `port_forward_local`
  - later-phase candidate
- `port_forward_remote`
  - unsupported in the first design pass
- `mount`
  - unsupported
- `agent_forward`
  - unsupported

Important design notes:

- AWS SSM should be modeled as a true connector, not as fake SSH
- the connector should consume EC2 inventory metadata rather than re-discovering identity out of band
- the first implementation should focus on shell and command execution
- file transfer should stay out of scope unless a clear native AWS-backed approach is defined

Recommended `connector.prepare` shape:

- `shell`
  - `plan.kind = "provider-managed"` or `plan.kind = "command"`
  - command plan may describe an `aws ssm start-session`-style invocation
- `exec`
  - `plan.kind = "provider-managed"` or structured command plan for SendCommand/session-backed exec

## Reference Capability Matrix

This matrix is a design target, not an implementation status table.

| Connector | shell | exec | exec_pty | upload | download | port_forward_local | port_forward_remote | mount | agent_forward |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| SSH | yes | yes | yes | yes | yes | yes | yes | conditional | yes |
| Telnet | yes | no | no | no | no | no | no | no | no |
| WinRM | conditional | yes | no | later | later | no | no | no | no |
| AWS SSM | yes | yes | conditional | no | no | later | no | no | no |

Recommended reading of the matrix:

- `yes`
  - should be modeled as supported in the first design
- `no`
  - should be modeled as unsupported
- `conditional`
  - requires target- or implementation-specific checks in `connector.describe`
- `later`
  - intentionally deferred from the first implementation wave

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

Recommended `plan.kind` values for the first design pass:

- `provider-managed`
  - the provider itself owns execution lifecycle
- `command`
  - the provider returns a structured local command invocation plan
- `builtin-ssh`
  - the provider indicates that normal SSH transport may be used after connector resolution

For the first implementation wave, `connector.describe` should come before `connector.prepare`.

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

Recommended AWS metadata contract for the first implementation:

- `instance_id`
- `region`
- `availability_zone`
- `platform`
- `private_ip`
- `public_ip`
- `tag.Name`
- `tag.<TagName>`

Recommended AWS SSM operation capability defaults:

- `shell`
  - supported for SSM-managed instances
- `exec`
  - supported
- `exec_pty`
  - supported when session manager PTY-like session is available
- `upload`
  - not supported in the first design pass
- `download`
  - not supported in the first design pass

This keeps the first connector scope focused on session and command execution rather than file transfer emulation.

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
