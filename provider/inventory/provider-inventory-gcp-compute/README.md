provider-inventory-gcp-compute Provider
=======================================

## About

The `gcp-compute` inventory provider lists running Compute Engine instances with the Google Compute Engine Go client and returns them as dynamic `server` entries.

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-inventory-gcp-compute"]

[provider.gcp]
plugin = "provider-inventory-gcp-compute"
enabled = true
capabilities = ["inventory"]
project = "example-project"
zone = "asia-northeast1-a"
credentials_file = "~/.config/gcloud/application_default_credentials.json"
server_name_template = "gcp:${name}"
note_template = "gcp ${zone} ${private_ip}"

[provider.gcp.match.web]
meta_in = ["label.role=web", "zone=*asia-northeast1-a"]
user = "ubuntu"
key = "~/.ssh/gcp-web.pem"

[provider.gcp.match.debian]
name_in = ["gcp:debian-*"]
user = "debian"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- Uses Google Application Default Credentials by default.
- `credentials_file` can be used to point at a specific service-account or ADC JSON file.
- `endpoint` and `scopes` can be overridden when needed.
- Uses private IP first, then public IP if needed.
- Only running instances are returned.
- `match` can override SSH settings per generated host.
- Available match metadata includes `name`, `id`, `zone`, `private_ip`, `public_ip`, and `label.<LabelName>`.
