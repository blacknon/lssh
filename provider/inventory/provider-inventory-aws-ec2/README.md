provider-inventory-aws-ec2 Provider
===================================

## About

The `aws-ec2` inventory provider lists running EC2 instances with the AWS SDK for Go and returns them as dynamic `server` entries.

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-inventory-aws-ec2"]

[provider.aws]
plugin = "provider-inventory-aws-ec2"
enabled = true
capabilities = ["inventory"]
regions = ["ap-northeast-1"]
profile = "default"
shared_config_files = ["~/.aws/config"]
shared_credentials_files = ["~/.aws/credentials"]
server_name_template = "aws:${tags.Name}"
note_template = "aws ${instance_id} ${private_ip}"

[provider.aws.match.web]
meta_in = ["tag.Role=web", "region=ap-northeast-1"]
user = "ec2-user"
key = "~/.ssh/aws-web.pem"

[provider.aws.match.ubuntu]
name_in = ["aws:ubuntu-*"]
user = "ubuntu"
key = "~/.ssh/aws-ubuntu.pem"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- Uses the AWS SDK default credential/config chain.
- `profile`, `shared_config_files`, and `shared_credentials_files` can be used to steer authentication.
- Uses private IP first, then public IP if needed.
- Only running instances are returned.
- `match` can override SSH settings per generated host.
- Available match metadata includes `region`, `zone`, `instance_id`, `private_ip`, `public_ip`, and `tag.<TagName>`.
