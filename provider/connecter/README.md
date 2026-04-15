Connecter Providers
===================

`connecter` providers are not implemented yet, but this directory is reserved for the design and future provider type that describes how a resolved target can actually be used.

The current spelling in the repository is `connecter`, and this document follows that existing naming.

## Why A Separate Provider Type May Be Needed

Some targets are not well described by ordinary SSH fields alone.

Examples:

- a target supports interactive shell login but not file transfer
- a target supports command execution only through an API
- a target supports a console session but not SFTP or SSHFS
- a target allows upload/download through a non-SSH transport

Concrete examples that fit this direction include:

- WinRM
- telnet
- serial console
- AWS SSM

In those cases, `inventory` alone is not enough. A separate provider type helps express what is actually possible for a target.

## Intended Responsibility

A connecter provider should answer:

- what operations are supported for this target
- how those operations should be invoked
- what limitations or prerequisites apply

This is different from:

- `inventory`
  - which discovers the target
- `secret`
  - which resolves credentials or tokens

## Relationship With Inventory

Although `connecter` is a separate provider type, it is expected to work closely with `inventory` in some environments.

Typical examples:

- an EC2 inventory provider returns `instance_id`, `region`, `platform`, or similar metadata
- an AWS SSM connecter provider uses that metadata to decide whether `shell` or `exec` can be offered
- a console-oriented inventory may return node, slot, or device identifiers that a serial connecter later consumes

This means a connecter provider may depend on inventory metadata without becoming an inventory provider itself.

The intended boundary is:

- `inventory`
  - discovers targets and emits stable metadata
- `connecter`
  - consumes target identity and metadata to describe supported operations

## Multi-Capability Plugin Model

The design does not require one executable per provider category.

Two shapes should both be considered valid:

- separate executables
  - `provider-inventory-aws-ec2`
  - `provider-connecter-aws-ssm`
- one executable with multiple capabilities
  - a single provider binary that exposes both `inventory` and `connecter`

The important point is that the contract stays explicit.
Even when one binary supports multiple capabilities, each capability should still have a clear role and a well-defined input/output boundary.

This is especially useful for environments like AWS where:

- inventory and connecter logic rely on the same upstream API family
- the same resource identifiers are needed in both phases
- splitting into separate binaries may add configuration overhead without improving clarity

## Recommended Capability Model

If implemented, a connecter provider should be capability-based.

Example capabilities:

- `shell`
- `exec`
- `pty_exec`
- `upload`
- `download`
- `mount`
- `port_forward_local`
- `port_forward_remote`
- `agent_forward`

The exact names can still change, but the main idea is that each `cmd/*` command should be able to ask whether its required operation is supported before attempting to run.

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

For example:

- a WinRM target may support `exec` but not `mount`
- a telnet target may support `shell` but not `upload`
- an AWS SSM target may support `shell` and `exec`, while transfer features may need a different path or remain unsupported

## Recommended Rollout Order

If this provider type is added later, the safest order is:

1. Document the config and result schema.
2. Add a read-only capability discovery method.
3. Use those capabilities to hide or reject unsupported commands cleanly.
4. Add runtime execution methods one category at a time.
5. Treat transfer and mount as separate phases, not as assumptions baked into shell support.

This keeps the first implementation small and prevents a single provider type from becoming a catch-all abstraction.

## Design Guidance For AWS SSM-Like Cases

AWS SSM is a good example of why this provider type exists.

A practical design may look like either of these:

- inventory and connecter are separate providers
  - inventory discovers EC2 instances
  - connecter decides how SSM-based access works for those instances
- one provider executable exposes both `inventory` and `connecter`
  - useful when both features rely on the same upstream API and metadata model

Either shape can work.
The recommendation is:

- keep the config and capability contracts separated
- allow one binary to implement multiple capabilities when that reduces duplication
- make the inventory metadata consumed by the connecter explicit and documented

That balance preserves the design boundary without forcing unnecessary implementation fragmentation.

## Non-Goals

A connecter provider should not become:

- a replacement for inventory discovery
- a general secret manager
- a wrapper around arbitrary local shell commands when an API/SDK approach is available

Its value is in describing and mediating transport behavior, not in absorbing every integration concern.
