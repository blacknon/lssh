provider-inventory-azure-compute Provider
=========================================

## About

The `azure-compute` inventory provider lists Azure Virtual Machines from Azure Resource Manager and returns them as dynamic `server` entries.

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-inventory-azure-compute"]

[provider.azure]
plugin = "provider-inventory-azure-compute"
enabled = true
capabilities = ["inventory"]
subscription_id = "00000000-0000-0000-0000-000000000000"
tenant_id = "11111111-1111-1111-1111-111111111111"
client_id = "22222222-2222-2222-2222-222222222222"
client_secret_env = "AZURE_CLIENT_SECRET"
resource_group = "rg-demo"
server_name_template = "azure:${name}"
note_template = "azure ${location} ${resource_group} ${power_state}"

[provider.azure.match.web]
meta_in = ["tag.Role=web", "power_state=running"]
user = "azureuser"
key = "~/.ssh/azure-web.pem"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- Uses Azure Resource Manager REST APIs.
- Authentication supports either `access_token` or service principal settings with `tenant_id`, `client_id`, and `client_secret`.
- `client_secret` and `access_token` can use `_env` or `_source` variants.
- `subscription_id` is required.
- `resource_group` is optional. If omitted, the provider lists VMs across the subscription.
- `endpoint` defaults to `https://management.azure.com`.
- `authority_host` defaults to `https://login.microsoftonline.com`.
- Uses private IP first, then public IP if needed.
- By default only VMs with `power_state=running` are returned.
- `include_stopped = true` widens the default status filter, or `statuses = ["running", "stopped", "deallocated"]` can be used explicitly.
- `include_tags` can copy selected Azure tags into generated server config as `tag_<name>`.
- `match` can override SSH settings per generated host.
- Available match metadata includes `id`, `name`, `subscription_id`, `resource_group`, `location`, `private_ip`, `public_ip`, `power_state`, `provisioning_state`, `os_type`, and `tag.<TagName>`.
