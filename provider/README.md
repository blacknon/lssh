Provider
========

## About

The `provider` directory contains external provider implementations used by `lssh`.
Providers are grouped by capability:

- [`inventory`](./inventory/README.md): generate `server` entries from cloud or API inventories
- [`secret`](./secret/README.md): resolve `*_ref` values just before connect

Each provider is a standalone executable that communicates with `lssh` over JSON via stdin/stdout.
