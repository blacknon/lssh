provider-mixed-aws-ec2 Provider
================================

## About

The `aws-ec2` mixed provider lists running EC2 instances with the AWS SDK for Go and returns them as dynamic `server` entries.
It also exposes an AWS SSM-based connector contract for those instances.

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

[provider.aws.match.ssm]
meta_in = ["tag.Connection=ssm"]
connector_name = "aws-ssm"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- `providers.max_parallel` limits how many inventory providers are queried at the same time.
  - unset or `0` means no explicit limit
  - inventory fetch is parallel, but merge order stays deterministic by provider order
- current plugin capabilities are `["inventory", "connector"]`.
- `plugin.describe` reports connector name `aws-ssm`.
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
- Available match metadata includes `region`, `zone`, `platform`, `instance_id`, `private_ip`, `public_ip`, and `tag.<TagName>`.
- `connector.describe` requires `instance_id` and `region`, which are emitted by this inventory provider.
- `connector.prepare` currently returns provider-managed AWS SSM plans for `shell`, `exec`, and `exec_pty`.
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
- optional connector tuning keys:
  - `ssm_shell_document`
  - `ssm_interactive_command_document`
- current runtime behavior:
  - `shell` is executed with `aws ssm start-session`
    - attach uses `aws ssm resume-session`
    - detach uses the AWS SDK `StartSession` API and returns the created session id
  - `exec` is executed with the AWS SDK via `SendCommand`
- to use `shell`, the local machine must have:
  - AWS CLI
  - Session Manager plugin for AWS CLI

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

Not recommended for the first implementation wave:

- `upload`
- `download`
- `mount`

Those can be designed later if there is a clear, native AWS SSM transfer model worth exposing.
