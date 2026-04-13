provider-secret-custom-script Provider
======================================

## About

The `custom-script` secret provider delegates secret resolution to an external script or command.

## Example

```toml
[providers]
paths = ["~/.config/lssh/providers/provider-secret-custom-script"]

[provider.custom]
plugin = "provider-secret-custom-script"
enabled = true
capabilities = ["secret"]
command = ["/path/to/resolve-secret"]

[server.ops]
addr = "10.0.0.40"
user = "ubuntu"
pass_ref = "custom:prod/ssh/password"
```

## Notes

- `providers.paths` is intended to list provider executable files.
- The provider executes `command` or `path`.
- The script receives:
  - `LSSH_PROVIDER_METHOD`
  - `LSSH_PROVIDER_REF`
  - `LSSH_PROVIDER_SERVER`
  - `LSSH_PROVIDER_FIELD`
- The script should print the resolved value to stdout.
