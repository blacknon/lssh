provider-inventory-ansible Provider
===================================

## About

The `ansible` inventory provider reads a local Ansible inventory file and turns it into dynamic `server` entries for `lssh`.

It reads inventory files directly instead of shelling out to `ansible-inventory`.

Supported input formats:

- INI inventory
- YAML inventory
- JSON inventory

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-inventory-ansible"]

[provider.ansible]
plugin = "provider-inventory-ansible"
enabled = true
capabilities = ["inventory"]
inventory_file = "~/.ansible/inventory/hosts.yml"
inventory_format = "auto"
server_name_template = "ansible:${inventory_hostname}"
note_template = "ansible ${groups}"

[provider.ansible.match.web]
meta_in = ["group.web=true"]
user = "ec2-user"

[provider.ansible.match.prod]
meta_in = ["var.env=prod"]
note_append = " [prod]"
```

Inventory-side SSH settings such as `ansible_host`, `ansible_user`, `ansible_port`, `ansible_password`, and `ansible_ssh_private_key_file` are imported into generated `server` entries.
If you want to override them per host group, use `provider.<name>.match.*`.

## Notes

- `inventory_file` is required.
- `inventory_format` can be `auto`, `ini`, `yaml`, or `json`. `auto` chooses from the file extension and falls back to `ini`.
- `include_groups` limits imported hosts to hosts that belong to at least one named group.
- `exclude_groups` removes hosts that belong to any named group.
- Group membership is inherited through `:children` and nested `children`.
- Host patterns like `web[01:03].example.com` are expanded for INI inventories.
- Generated metadata includes `inventory_hostname`, `groups`, `group.<group>=true`, `var.<inventory_var>=...`, `ansible_host`, `ansible_user`, and `ansible_port`.
