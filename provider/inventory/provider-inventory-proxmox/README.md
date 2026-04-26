provider-inventory-proxmox Provider
===================================

## About

The `proxmox` inventory provider connects to the Proxmox VE API and returns VM inventory as dynamic `server` entries.

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-inventory-proxmox"]
debug_log = "~/.cache/lssh/provider-debug.log"

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
statuses = ["running"]
os_families = ["non-windows"]

[provider.proxmox.when]
local_ip_in = ["192.168.0.0/16"]
gateway_in = ["192.168.1.1"]

[provider.proxmox.match.node_a]
meta_in = ["node=pve-a"]
user = "root"
key = "~/.ssh/pve-node-a.pem"

[provider.proxmox.match.db]
name_in = ["pve:*:db-*"]
user = "postgres"

[provider.proxmox.match.office]
meta_in = ["node=sv-pve0*"]
note_append = " [office:${meta:node}]"

[provider.proxmox.match.windows]
meta_in = ["os_family=windows"]
note_template = "${note} [windows ${meta:ostype}]"
```

Provider connection settings such as `host`, `port`, `username`, `token_*`, and template options are for the Proxmox API itself.
Use `match.*` entries to set SSH-facing defaults like `user`, `key`, or SSH `port` for generated hosts.
Use `when.*` when the provider itself should only run on specific client networks or environments.

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
- `providers.debug_log` or `provider.<name>.debug_log` can be used to append provider request / response / stderr details to a local file while debugging.
- Uses the Proxmox VE API endpoint `/api2/json/cluster/resources?type=vm`.
- Supports either `token_id` + `token_secret` or `username` + `password`.
- `token_secret` and `password` can use `_env` or `_source` variants.
- `host` is required. `scheme` defaults to `https`. `port` defaults to `8006`.
- `when.*` can be used to enable the whole provider only when the client matches conditions such as `local_ip_in`, `gateway_in`, `os_in`, `term_in`, or `env_in`.
- `insecure = true` skips TLS certificate verification.
- `addr_template` controls the SSH target address. Use it when the guest name or another naming rule should become `server.addr`.
- `node_addr_prefix` remains available if the SSH target should be the Proxmox node rather than the guest.
- `vm_types` can be used to limit the result to `qemu`, `lxc`, or both.
- `statuses` can be used to choose which VM states are imported, for example `["running"]` or `["running", "stopped"]`.
- `os_families` can be used to limit imported guests to `windows` or `non-windows`. The provider inspects QEMU guest `ostype` so `os_family` / `ostype` metadata can also be used from `match.meta_in`. If `ostype` cannot be fetched for a QEMU guest, `os_family=unknown` is set.
- By default only running, non-template VMs are returned. `include_stopped = true` remains available as a backward-compatible shortcut for including `stopped`, and `include_templates = true` can widen that.
- `match` can override SSH settings per generated host.
- `match.note_template` can rebuild the imported note with `${note}`, `${provider}`, `${server}`, and `${meta:<key>}` placeholders.
- `match.note_append` can append extra text to the imported note using the same placeholders.
- Available match metadata includes `name`, `node`, `vmid`, `type`, `id`, `status`, `os_family`, and `ostype` for QEMU guests. `os_family` can be `windows`, `non-windows`, or `unknown`.
