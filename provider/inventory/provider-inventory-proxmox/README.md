provider-inventory-proxmox Provider
===================================

## About

The `proxmox` inventory provider connects to the Proxmox VE API and returns VM inventory as dynamic `server` entries.

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-inventory-proxmox"]

[provider.proxmox]
plugin = "provider-inventory-proxmox"
enabled = true
capabilities = ["inventory"]
host = "pve.example.local"
port = "8006"
insecure = true
token_id = "root@pam!lssh"
token_secret_env = "PVE_TOKEN_SECRET"
server_name_template = "pve:${node}:${name}"
addr_template = "${name}"
note_template = "proxmox ${node} vmid=${vmid}"
vm_types = ["qemu"]

[provider.proxmox.match.node_a]
meta_in = ["node=pve-a"]
user = "root"
key = "~/.ssh/pve-node-a.pem"

[provider.proxmox.match.db]
name_in = ["pve:*:db-*"]
user = "postgres"
```

Password-based auth is also supported:

```toml
[provider.proxmox]
plugin = "provider-inventory-proxmox"
enabled = true
capabilities = ["inventory"]
host = "pve.example.local"
username = "root@pam"
password_env = "PVE_PASSWORD"
addr_template = "${name}.example.local"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- Uses the Proxmox VE API endpoint `/api2/json/cluster/resources?type=vm`.
- Supports either `token_id` + `token_secret` or `username` + `password`.
- `token_secret` and `password` can use `_env` or `_source` variants.
- `host` is required. `scheme` defaults to `https`. `port` defaults to `8006`.
- `insecure = true` skips TLS certificate verification.
- `addr_template` controls the SSH target address. Use it when the guest name or another naming rule should become `server.addr`.
- `node_addr_prefix` remains available if the SSH target should be the Proxmox node rather than the guest.
- `vm_types` can be used to limit the result to `qemu`, `lxc`, or both.
- By default only running, non-template VMs are returned. `include_stopped = true` and `include_templates = true` can widen that.
- `match` can override SSH settings per generated host.
- Available match metadata includes `name`, `node`, `vmid`, `type`, `id`, and `status`.
