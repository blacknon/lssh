provider-mixed-azure-compute Provider
=========================================

## About

The `azure-compute` mixed provider lists Azure Virtual Machines from Azure Resource Manager and returns them as dynamic `server` entries.
It also exposes an `azure-bastion` connector for Bastion-based SSH access.

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-mixed-azure-compute"]

[provider.azure]
plugin = "provider-mixed-azure-compute"
enabled = true
capabilities = ["inventory", "connector"]
resource_group = "rg-demo"
timeout = "30s"
addr_strategy = "public_first"
server_name_template = "azure:${name}"
note_template = "azure ${location} ${resource_group} ${power_state}"

[provider.azure.match.direct_ssh]
meta_in = ["tag.Role=web", "tag.Connection=ssh", "power_state=running"]
connector_name = "ssh"
user = "azureuser"
key = "~/.ssh/azure-web.pem"

[provider.azure.match.bastion_sdk]
meta_in = ["tag.Role=web", "tag.Connection=bastion-sdk", "power_state=running"]
connector_name = "azure-bastion"
bastion_runtime = "sdk"
bastion_name = "shared-bastion"
bastion_resource_group = "rg-network"
user = "azureuser"
key = "~/.ssh/azure-web.pem"

[provider.azure.match.bastion_command]
meta_in = ["tag.Role=web", "tag.Connection=bastion-command", "power_state=running"]
connector_name = "azure-bastion"
bastion_runtime = "command"
bastion_name = "shared-bastion"
bastion_resource_group = "rg-network"
bastion_auth_type = "AAD"
user = "azureuser"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- Uses Azure Resource Manager REST APIs.
- `plugin.describe` reports connector name `azure-bastion`.
- Authentication supports:
  - `access_token` / `access_token_env` / `access_token_source`
  - service principal settings with `tenant_id`, `client_id`, and `client_secret`
  - Azure SDK `DefaultAzureCredential`
- `DefaultAzureCredential` lets the provider reuse existing Azure auth such as:
  - `az login`
  - `AZURE_CLIENT_ID` / `AZURE_TENANT_ID` / `AZURE_CLIENT_SECRET`
  - managed identity and other standard Azure SDK credential sources
- `client_secret` and `access_token` can use `_env` or `_source` variants.
- `subscription_id` can be supplied by:
  - `subscription_id` / `subscription_id_env` / `subscription_id_source`
  - `AZURE_SUBSCRIPTION_ID`
  - automatic discovery when Azure returns exactly one enabled subscription
- when multiple enabled subscriptions are available, set one of the values above explicitly.
- `resource_group` is optional. If omitted, the provider lists VMs across the subscription.
- `bastion_name` is required when using the `azure-bastion` connector.
- `bastion_resource_group` is required when using the `azure-bastion` connector.
- `bastion_runtime` controls the connector runtime.
  - `sdk` (default): provider-managed SSH transport for shell, exec, SFTP, mount, and local forwarding
  - `command`: Azure CLI based Bastion SSH / local forward transport
- `bastion_auth_type` defaults to `AAD` when the command runtime is used without `key`.
- when `sdk` and `command` hosts are mixed, set `bastion_runtime`, `bastion_name`, and `bastion_resource_group` on each server or `match` entry instead of provider-wide.
- `endpoint` defaults to `https://management.azure.com`.
- when using Azure SDK default credentials against sovereign clouds, set the standard Azure SDK environment variables such as `AZURE_AUTHORITY_HOST` as needed.
- `addr_strategy` controls how generated `addr` is chosen.
  - `private_first` (default)
  - `public_first`
  - `private_only`
  - `public_only`
- By default only VMs with `power_state=running` are returned.
- `include_stopped = true` widens the default status filter, or `statuses = ["running", "stopped", "deallocated"]` can be used explicitly.
- `include_tags` can copy selected Azure tags into generated server config as `tag_<name>`.
- `match` can override SSH settings per generated host.
- Available match metadata includes `id`, `name`, `subscription_id`, `resource_group`, `location`, `private_ip`, `public_ip`, `power_state`, `provisioning_state`, `os_type`, and `tag.<TagName>`.
- `connector.describe` / `connector.prepare` require target resource metadata from this provider.
- `shell` and `port_forward_local` are available in both runtimes.
- `exec`, `exec_pty`, `sftp_transport`, `upload`, `download`, `mount`, and internal `tcp_dial_transport` are available in `sdk` runtime.

## Per-server Example

```toml
[provider.azure.match.direct_ssh]
meta_in = ["tag.Connection=ssh"]
connector_name = "ssh"
user = "azureuser"
key = "~/.ssh/azure-web.pem"

[provider.azure.match.bastion_sdk]
meta_in = ["tag.Connection=bastion-sdk"]
connector_name = "azure-bastion"
bastion_runtime = "sdk"
bastion_name = "shared-bastion"
bastion_resource_group = "rg-network"
user = "azureuser"
key = "~/.ssh/azure-web.pem"

[provider.azure.match.bastion_command]
meta_in = ["tag.Connection=bastion-command"]
connector_name = "azure-bastion"
bastion_runtime = "command"
bastion_name = "shared-bastion"
bastion_resource_group = "rg-network"
bastion_auth_type = "AAD"
user = "azureuser"
```

See [example/provider-azure-bastion.toml](/Users/blacknon/_go/src/github.com/blacknon/lssh/example/provider-azure-bastion.toml).
