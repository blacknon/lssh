Configuration
=============

This document collects the shared configuration features used across the `lssh` suite.
For command-specific behavior such as forwarding examples or local rc usage, see [`cmd/lssh/README.md`](../cmd/lssh/README.md).

## TUI navigation and key bindings

Most `lssh` commands open the same host selection TUI when you do not pass hosts with `-H`.
You can filter the list by typing, then move and confirm the selection from the keyboard.

- <kbd>Up</kbd> / <kbd>Down</kbd>: move the cursor one line
- <kbd>Left</kbd> / <kbd>Right</kbd>: move between pages
- <kbd>Tab</kbd>: toggle the current host and move to the next line in multi-select screens
- <kbd>Ctrl</kbd> + <kbd>A</kbd>: select or unselect all visible hosts in multi-select screens
- <kbd>Backspace</kbd>: delete one character from the current filter
- <kbd>Space</kbd>: insert a space into the filter text
- <kbd>Enter</kbd>: confirm the current selection
- <kbd>Esc</kbd> or `Ctrl + C`: quit immediately

Mouse left click also moves the cursor to the clicked line.

`lsmon` adds one extra key binding after startup:

- `Ctrl + X`: toggle the top-panel view for the currently selected host

## Shared host inventory

`lssh` reads `~/.lssh.conf` by default.
You can define shared settings in `[common]` and host entries in `[server.<name>]`.
Values in `[server.<name>]` override values from `[common]`.

Minimal example:

```toml
[common]
user = "demo"
port = "22"
key = "~/.ssh/id_rsa"

[server.dev]
addr = "192.168.100.10"
note = "development server"
```

At minimum, a server entry needs `addr`, `user`, and authentication settings such as `pass`, `key`, `cert`, `pkcs11`, or `agentauth`.

## Keepalive settings

You can configure SSH keepalive probes with `alive_interval` and `alive_max`.
These behave like OpenSSH `ServerAliveInterval` and `ServerAliveCountMax`.

```toml
[common]
alive_interval = 10
alive_max = 3
```

With the example above, `lssh` sends a keepalive request every 10 seconds and closes the connection after 3 consecutive failures.

## OpenSSH config import

Load and use `~/.ssh/config` by default.
`ProxyCommand` can also be used.

You can also specify and read another path.
In addition to the path, server config items can be specified and applied collectively.

```toml
[sshconfig.default]
path = "~/.ssh/config"
pre_cmd = 'printf "\033]50;SetProfile=local\a"'
post_cmd = 'printf "\033]50;SetProfile=Default\a"'
```

## Split config into multiple files

You can include server settings in other files.
Each included file can also define its own `[common]` settings.

`~/.lssh.conf`:

```toml
[includes]
path = [
     "~/.lssh.d/home.conf"
    ,"~/.lssh.d/cloud.conf"
]
```

`~/.lssh.d/home.conf`:

```toml
[common]
pre_cmd = 'printf "\033]50;SetProfile=dq\a"'
post_cmd = 'printf "\033]50;SetProfile=Default\a"'
ssh_agent_key = ["~/.ssh/id_rsa"]
ssh_agent = false
user = "user"
key = "~/.ssh/id_rsa"
pkcs11provider = "/usr/local/lib/opensc-pkcs11.so"

[server.Server1]
addr = "172.16.200.1"
note = "TEST Server1"
local_rc = "yes"

[server.Server2]
addr = "172.16.200.2"
note = "TEST Server2"
local_rc = "yes"
```

Priority order:

`[server.<name>]` > `[common]` in included file > `[common]` in `~/.lssh.conf`

## Multi-stage proxy

`lssh` supports multiple proxy styles:

- `http`
- `socks5`
- `ssh`
- `ProxyCommand`

`http` proxy example:

```toml
[proxy.HttpProxy]
addr = "example.com"
port = "8080"

[server.overHttpProxy]
addr = "over-http-proxy.com"
key  = "/path/to/private_key"
note = "connect use http proxy"
proxy = "HttpProxy"
proxy_type = "http"
```

`socks5` proxy example:

```toml
[proxy.Socks5Proxy]
addr = "example.com"
port = "54321"

[server.overSocks5Proxy]
addr = "192.168.10.101"
key  = "/path/to/private_key"
note = "connect use socks5 proxy"
proxy = "Socks5Proxy"
proxy_type = "socks5"
```

`ssh` proxy example:

```toml
[server.sshProxyServer]
addr = "192.168.100.200"
key  = "/path/to/private_key"
note = "proxy server"

[server.overProxyServer]
addr = "192.168.10.10"
key  = "/path/to/private_key"
note = "connect use ssh proxy"
proxy = "sshProxyServer"

[server.overProxyServer2]
addr = "192.168.10.100"
key  = "/path/to/private_key"
note = "connect use ssh proxy(multiple)"
proxy = "overProxyServer"
```

`ProxyCommand` example:

```toml
[server.ProxyCommand]
addr = "192.168.10.20"
key  = "/path/to/private_key"
note = "connect use ssh proxy(multiple)"
proxy_cmd = "ssh -W %h:%p proxy"
```

## Available authentication methods

Supported methods:

- Password auth
- Public key auth
- Certificate auth
- PKCS#11 auth
- `ssh-agent` auth

Password auth example:

```toml
[server.PasswordAuth]
addr = "password_auth.local"
user = "user"
pass = "Password"
note = "password auth server"
```

Public key auth example:

```toml
[server.PublicKeyAuth]
addr = "pubkey_auth.local"
user = "user"
key = "~/path/to/key"
note = "public key auth server"

[server.PublicKeyAuth_with_passwd]
addr = "password_auth.local"
user = "user"
key = "~/path/to/key"
keypass = "passphrase"
note = "public key auth server with passphrase"
```

Certificate auth example:

```toml
[server.CertAuth]
addr = "cert_auth.local"
user = "user"
cert = "~/path/to/cert"
certkey = "~/path/to/certkey"
note = "certificate auth server"

[server.CertAuth_with_passwd]
addr = "cert_auth.local"
user = "user"
cert = "~/path/to/cert"
certkey = "~/path/to/certkey"
certkeypass = "passphrase"
note = "certificate auth server with passphrase"
```

PKCS#11 auth example:

```toml
[server.PKCS11Auth]
addr = "pkcs11_auth.local"
user = "user"
pkcs11provider = "/usr/local/lib/opensc-pkcs11.so"
pkcs11 = true
note = "PKCS11 auth server"

[server.PKCS11Auth_with_PIN]
addr = "pkcs11_auth.local"
user = "user"
pkcs11provider = "/usr/local/lib/opensc-pkcs11.so"
pkcs11 = true
pkcs11pin = "123456"
note = "PKCS11 auth server"
```

`ssh-agent` auth example:

```toml
[server.SshAgentAuth]
addr = "agent_auth.local"
user = "user"
agentauth = true
note = "ssh-agent auth server"
```

## Check KnownHosts

Enable `known_hosts` verification with `check_known_hosts = true`.
You can also specify the file to use with `known_hosts_files`.

```toml
[server.CheckKnownHosts]
addr = "check_known_hosts.local"
user = "user"
check_known_hosts = true
note = "check known hosts example"

[server.CheckKnownHostsToOriginalFile]
addr = "check_known_hosts.local"
user = "user"
check_known_hosts = true
known_hosts_files = ["/path/to/known_hosts"]
note = "check known hosts example"
```

## ControlMaster / ControlPersist

You can reuse SSH sessions with OpenSSH-style ControlMaster settings.
This is useful when multiple operations connect to the same host repeatedly.

```toml
[common]
control_master = true
control_path = "/tmp/lssh-control-%h-%p-%r"
control_persist = "10m"

[server.controlmaster]
addr = "192.168.100.110"
user = "demo"
key = "~/.ssh/id_rsa"
note = "reuse ssh session"
```

## Conditional overrides with `match`

Use `[server.<name>.match.<branch>]` when you want to override only part of a host configuration under specific conditions.
Each branch can change fields such as `proxy`, `user`, `port`, `note`, or `ignore`.
All matching branches are evaluated in ascending `priority` order, and later branches overwrite earlier ones.

This lets you tailor the same logical host to your current network, client OS, terminal, or environment.

```toml
[server.app]
addr = "192.168.100.50"
user = "demo"
key = "~/.ssh/id_rsa"
note = "direct by default"

[server.app.match.office_network]
priority = 1
when.local_ip_in = ["192.168.100.0/24"]
proxy = "ssh-bastion"
note = "use bastion from office network"

[server.app.match.macos]
priority = 50
when.os_in = ["darwin"]
when.term_in = ["iterm2"]
when.env_in = ["SSH_AUTH_SOCK"]
when.env_value_in = ["TERM_PROGRAM=iTerm.app"]
user = "demo-mac"
note = "prefer macOS + iTerm2 settings"

[server.app.match.outside_office]
priority = 90
when.local_ip_not_in = ["192.168.100.0/24"]
ignore = true
```

In this example:

- `app` connects directly by default
- inside `192.168.100.0/24`, `office_network` routes through `ssh-bastion`
- on macOS inside iTerm2, the later `macos` branch also overrides `user` and `note`
- outside that network, `outside_office` hides the host from selection

Available `when.*` keys:

- `local_ip_in`, `local_ip_not_in`
- `gateway_in`, `gateway_not_in`
- `username_in`, `username_not_in`
- `hostname_in`, `hostname_not_in`
- `os_in`, `os_not_in`
- `term_in`, `term_not_in`
- `env_in`, `env_not_in`
- `env_value_in`, `env_value_not_in`

Notes:

- Lower `priority` values are applied first
- Higher `priority` values win when the same field is set multiple times
- `os_*` matches `runtime.GOOS` values such as `darwin`, `linux`, or `windows`
- `term_*` mainly matches normalized values from `TERM_PROGRAM` and `TERM` such as `iterm2`, `apple_terminal`, `xterm`, or `tmux`
- `env_*` checks whether the named environment variables exist
- `env_value_*` matches exact `KEY=value` pairs
