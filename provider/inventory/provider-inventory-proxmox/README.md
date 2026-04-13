provider-inventory-proxmox Provider
===================================

## About

The `proxmox` inventory provider lists running VMs with `pvesh` and returns them as dynamic `server` entries.

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-inventory-proxmox"]

[provider.proxmox]
plugin = "provider-inventory-proxmox"
enabled = true
capabilities = ["inventory"]
server_name_template = "pve:${node}:${name}"
node_addr_prefix = "pve-"
note_template = "proxmox ${node} vmid=${vmid}"

[provider.proxmox.match.node_a]
meta_in = ["node=pve-a"]
user = "root"
key = "~/.ssh/pve-node-a.pem"

[provider.proxmox.match.db]
name_in = ["pve:*:db-*"]
user = "postgres"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- Requires `pvesh`.
- The current implementation lists running VMs from `/cluster/resources`.
- `node_addr_prefix` can be used when the SSH target is the node rather than the guest itself.
- `match` can override SSH settings per generated host.
- Available match metadata includes `name`, `node`, and `vmid`.
