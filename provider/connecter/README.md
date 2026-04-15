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

## Recommended Rollout Order

If this provider type is added later, the safest order is:

1. Document the config and result schema.
2. Add a read-only capability discovery method.
3. Use those capabilities to hide or reject unsupported commands cleanly.
4. Add runtime execution methods one category at a time.
5. Treat transfer and mount as separate phases, not as assumptions baked into shell support.

This keeps the first implementation small and prevents a single provider type from becoming a catch-all abstraction.

## Non-Goals

A connecter provider should not become:

- a replacement for inventory discovery
- a general secret manager
- a wrapper around arbitrary local shell commands when an API/SDK approach is available

Its value is in describing and mediating transport behavior, not in absorbing every integration concern.
