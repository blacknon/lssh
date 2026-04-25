provider-mixed-gcp-compute Provider
=======================================

## About

The `gcp-compute` mixed provider lists running Compute Engine instances with the Google Compute Engine Go client and returns them as dynamic `server` entries.
It also exposes a `gcp-iap` connector for IAP TCP forwarding based SSH access.

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-mixed-gcp-compute"]
max_parallel = 4

[provider.gcp]
plugin = "provider-mixed-gcp-compute"
enabled = true
capabilities = ["inventory", "connector"]
project = "example-project"
credentials_file = "~/.config/gcloud/application_default_credentials.json"
addr_strategy = "public_first"
server_name_template = "gcp:${name}"
note_template = "gcp ${zone} ${private_ip}"

[provider.gcp.match.web_ssh]
meta_in = ["label.role=web", "label.connection=ssh"]
connector_name = "ssh"
user = "ubuntu"
key = "~/.ssh/gcp-web.pem"

[provider.gcp.match.web_sdk]
meta_in = ["label.role=web", "label.connection=iap-sdk"]
connector_name = "gcp-iap"
iap_runtime = "sdk"
zone = "asia-northeast1-a"
user = "ubuntu"
key = "~/.ssh/gcp-web.pem"

[provider.gcp.match.web_command]
meta_in = ["label.role=web", "label.connection=iap-command"]
connector_name = "gcp-iap"
iap_runtime = "command"
zone = "asia-northeast1-b"
user = "ubuntu"
key = "~/.ssh/gcp-web.pem"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- `providers.max_parallel` limits simultaneous `inventory.list` calls across configured providers.
  - unset or `0` means no explicit limit
- Uses Google Application Default Credentials by default.
- `credentials_file` can be used to point at a specific service-account or ADC JSON file.
- `endpoint` and `scopes` can be overridden when needed.
- `plugin.describe` reports connector name `gcp-iap`.
- `iap_runtime` controls the connector runtime.
  - `sdk` (default): provider-managed IAP WebSocket + SSH transport for shell, exec, SFTP, mount, and local forwarding
  - `command`: `gcloud compute ssh --tunnel-through-iap` based transport
- `sdk` runtime does not require `gcloud` in `PATH`.
- when `sdk` and `command` hosts are mixed, set `iap_runtime` on each server or `match` entry instead of provider-wide.
- `addr_strategy` controls how generated `addr` is chosen.
  - `private_first` (default)
  - `public_first`
  - `private_only`
  - `public_only`
- Only running instances are returned.
- `match` can override SSH settings per generated host.
- Available match metadata includes `name`, `id`, `project`, `zone`, `private_ip`, `public_ip`, and `label.<LabelName>`.
- `connector.describe` / `connector.prepare` require `project`, `zone`, and instance metadata from this provider.
- `shell` and `port_forward_local` are available in both runtimes.
- `exec`, `exec_pty`, `sftp_transport`, `upload`, `download`, `mount`, and internal `tcp_dial_transport` are available in `sdk` runtime.

## Per-server Example

```toml
[provider.gcp.match.web_ssh]
meta_in = ["label.connection=ssh"]
connector_name = "ssh"
user = "ubuntu"
key = "~/.ssh/gcp-web.pem"

[provider.gcp.match.web_sdk]
meta_in = ["label.connection=iap-sdk"]
connector_name = "gcp-iap"
iap_runtime = "sdk"
zone = "asia-northeast1-a"
user = "ubuntu"
key = "~/.ssh/gcp-web.pem"

[provider.gcp.match.web_command]
meta_in = ["label.connection=iap-command"]
connector_name = "gcp-iap"
iap_runtime = "command"
zone = "asia-northeast1-b"
user = "ubuntu"
key = "~/.ssh/gcp-web.pem"
```

See [example/provider-gcp-iap.toml](/Users/blacknon/_go/src/github.com/blacknon/lssh/example/provider-gcp-iap.toml).
