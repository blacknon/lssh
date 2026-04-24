provider-mixed-aws-ec2 Provider
================================

## About

The `aws-ec2` mixed provider lists running EC2 instances with the AWS SDK for Go and returns them as dynamic `server` entries.
It also exposes AWS connector contracts for those instances.

This README describes the current mixed implementation.

Planned direction:

- plugin name
  - `provider-mixed-aws-ec2`
- provider categories
  - `inventory`
  - `connector`
- connector backend
  - AWS SSM
  - later connector modes may also include direct SSH or bastion-backed SSH flows

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-mixed-aws-ec2"]
max_parallel = 4

[provider.aws]
plugin = "provider-mixed-aws-ec2"
enabled = true
capabilities = ["inventory", "connector"]
regions = ["ap-northeast-1"]
profile = "default"
shared_config_files = ["~/.aws/config"]
shared_credentials_files = ["~/.aws/credentials"]
addr_strategy = "public_first"
server_name_template = "aws:${tags.Name}"
note_template = "aws ${instance_id} ${private_ip}"
ssm_require_online = true

[provider.aws.match.web]
meta_in = ["tag.Role=web", "region=ap-northeast-1"]
connector_name = "ssh"
user = "ec2-user"
key = "~/.ssh/aws-web.pem"

[provider.aws.match.ssm_plugin]
meta_in = ["tag.Connection=ssm-plugin"]
connector_name = "aws-ssm"
ssm_shell_runtime = "plugin"
ssm_port_forward_runtime = "plugin"

[provider.aws.match.eice_sdk]
meta_in = ["tag.Connection=eice-sdk"]
connector_name = "aws-eice"
eice_runtime = "sdk"
instance_connect_endpoint_id = "eice-0123456789abcdef0"
private_ip_address = "10.0.1.10"
user = "ec2-user"
key = "~/.ssh/aws-eice.pem"


[provider.aws.match.eice_command]
meta_in = ["tag.Connection=eice-command"]
connector_name = "aws-eice"
eice_runtime = "command"
instance_connect_endpoint_dns_name = "eice-demo.ap-northeast-1.aws"
private_ip_address = "10.0.2.10"
user = "ec2-user"
key = "~/.ssh/aws-eice.pem"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- `providers.max_parallel` limits how many inventory providers are queried at the same time.
  - unset or `0` means no explicit limit
  - inventory fetch is parallel, but merge order stays deterministic by provider order
- current plugin capabilities are `["inventory", "connector"]`.
- `plugin.describe` reports connector names `aws-ssm` and `aws-eice`.
- inventory is implemented by `inventory.list`.
- connector is currently implemented by `connector.describe` and `connector.prepare`.
- Uses the AWS SDK default credential/config chain.
- `profile`, `shared_config_files`, and `shared_credentials_files` can be used to steer authentication.
- `addr_strategy` controls how generated `addr` is chosen.
  - `private_first` (default)
  - `public_first`
  - `private_only`
  - `public_only`
- Only running instances are returned.
- `match` can override SSH settings per generated host, including `connector_name`.
- `connector_name = "ssh"` forces the built-in go-sshlib path instead of the provider connector.
- `default_connector_name` can be used to select the provider connector when `match.connector_name` is omitted.
  - set `default_connector_name = "aws-ssm"` to preserve previous single-connector behavior
- when `aws-ssm` and `aws-eice` hosts are mixed, prefer setting `connector_name` and runtime keys on each server or `match` entry.
- Available match metadata includes `region`, `zone`, `platform`, `instance_id`, `private_ip`, `public_ip`, and `tag.<TagName>`.
- `connector.describe` requires connector-specific metadata emitted by this inventory provider.
- `aws-ssm` and `aws-eice` each implement `connector.describe` and `connector.prepare`.
- future AWS connector expansion may include:
  - direct SSH
  - bastion-backed SSH
  - OpenSSH-transport-based file operations layered in Go
- for `shell`, attach/detach are represented as operation options rather than separate subcommands.
  - `attach=true` with `session_id=<id>` resumes an existing SSM session
  - `detach=true` starts a shell session without attaching and returns a session id
  - `attach` and `detach` are mutually exclusive
- `ssm_require_online` defaults to `true`.
  - when enabled, the connector requires the target instance to be online in AWS Systems Manager
- `ssm_shell_runtime` controls how `shell` is executed.
  - `plugin` (default): use `aws ssm start-session`
  - `native`: use the experimental built-in Go runtime for plain shell start
    - `localrc` is supported only in this mode
- `ssm_port_forward_runtime` controls how local port forwarding is executed.
  - if omitted, it follows `ssm_shell_runtime`
  - `plugin`: use `aws ssm start-session` with the port forwarding document
  - `native`: use the experimental built-in Go runtime for local and dynamic forwarding
- optional connector tuning keys:
  - `ssm_shell_document`
  - `ssm_interactive_command_document`
  - `ssm_port_forward_document`
  - `instance_connect_endpoint_id`
  - `instance_connect_endpoint_dns_name`
  - `private_ip_address`
  - `eice_runtime`
- current runtime behavior:
  - `shell` is executed with `aws ssm start-session`
    - attach uses `aws ssm resume-session`
    - detach uses the AWS SDK `StartSession` API and returns the created session id
  - `shell` with `ssm_shell_runtime = "native"` currently supports only a plain start session
    - attach/detach still use the plugin runtime
    - `localrc` is executed by sending the generated startup command through the native session
  - `exec` is executed with the AWS SDK via `SendCommand`
    - when `ssm_shell_runtime = "native"` and the caller uses the connector stream path, `lspipe --raw` can stream stdin/stdout over the native runtime for Linux targets
  - `port_forward_local` supports both `plugin` and `native`
    - `lssh -L ...` works for `connector_name = "aws-ssm"` hosts
    - only one TCP local forward is supported in the first wave
    - bind address must stay on localhost / loopback
    - AWS SSM runs this as a forwarding-only session, so `-N` and `localrc` are ignored
    - in `native` mode, each accepted local TCP connection uses its own SSM session
    - X11 forwarding is still unsupported
  - dynamic port forwarding (`lssh -D ...`) supports both `plugin` and `native`
    - implemented as a local SOCKS5 listener plus one SSM port forwarding session per SOCKS connection
    - when `lssh` is used in shell mode, `-D` keeps the interactive shell open unless `-N` is specified
    - current `native` mode uses the AWS CLI/session-manager-plugin transport for each SOCKS connection while the built-in port-session path catches up with newer agent behavior
    - only SOCKS5 CONNECT without authentication is supported in the first wave
  - HTTP dynamic port forwarding (`lssh -d ...`) supports both `plugin` and `native`
    - implemented as a local HTTP proxy listener plus one SSM port forwarding session per request
    - when `lssh` is used in shell mode, `-d` keeps the interactive shell open unless `-N` is specified
    - current `native` mode uses the AWS CLI/session-manager-plugin transport for each HTTP proxy request
    - ordinary HTTP requests and HTTPS `CONNECT` are supported in the first wave
  - reverse / HTTP reverse / NFS / SMB forwarding still return explicit unsupported errors for `aws-ssm`
- to use `shell`, the local machine must have:
  - AWS CLI
  - Session Manager plugin for AWS CLI

## AWS EICE Connector

- `connector_name = "aws-eice"` enables EC2 Instance Connect Endpoint transport.
- `eice_runtime` controls the connector runtime.
  - `sdk` (default): provider-managed SSH transport for shell, exec, SFTP, mount, and local forwarding
  - `command`: OpenSSH `ProxyCommand` using `aws ec2-instance-connect open-tunnel`
- `aws-eice` requires:
  - `instance_id`
  - `region`
  - one of `instance_connect_endpoint_id` or `instance_connect_endpoint_dns_name`
  - target private address information, typically `private_ip_address`

Per-server example:

```toml
[provider.aws.match.eice_sdk]
meta_in = ["tag.Connection=eice-sdk"]
connector_name = "aws-eice"
eice_runtime = "sdk"
instance_connect_endpoint_id = "eice-0123456789abcdef0"
private_ip_address = "10.0.1.10"
user = "ec2-user"
key = "~/.ssh/aws-eice.pem"

[provider.aws.match.eice_command]
meta_in = ["tag.Connection=eice-command"]
connector_name = "aws-eice"
eice_runtime = "command"
instance_connect_endpoint_dns_name = "eice-demo.ap-northeast-1.aws"
private_ip_address = "10.0.2.10"
user = "ec2-user"
key = "~/.ssh/aws-eice.pem"
```

See [example/provider-aws-eice.toml](/Users/blacknon/_go/src/github.com/blacknon/lssh/example/provider-aws-eice.toml).

Example stream transfer with `lspipe`:

```bash
tar czf - ./dist | lspipe -h aws:ssm-host --raw 'tar xzf - -C /srv/app'
lspipe -h aws:ssm-host --raw 'tar czf - /srv/app' > app.tar.gz
```

## AWS SSM Connector Contract

The AWS SSM connector consumes inventory metadata from this provider rather than rediscovering instance identity.

Recommended connector-facing metadata:

- `instance_id`
- `region`
- `zone`
- `private_ip`
- `public_ip`
- `tag.Name`
- `tag.<TagName>`

Current operation capabilities for the AWS SSM connector layer:

- `shell`
- `exec`
- `exec_pty`
- `port_forward_local`
- internal `tcp_dial_transport` used by dynamic forwarding

Not recommended for the first implementation wave:

- `upload`
- `download`
- `mount`

Those can be designed later if there is a clear, native AWS SSM transfer model worth exposing.
