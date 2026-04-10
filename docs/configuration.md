Configuration
=============

This document collects the shared configuration features used across the `lssh` suite.
For command-specific behavior such as forwarding examples or local rc usage, see [`cmd/lssh/README.md`](../cmd/lssh/README.md).

## Configuration snippets

The snippets below collect the configuration patterns currently covered in this document.
Copy only the blocks you need into your `~/.lssh.conf`.

Base inventory:

```toml
[common]
user = "demo"
port = "22"
key = "~/.ssh/id_rsa"

[server.dev]
addr = "192.168.100.10"
note = "development server"
```

Logging:

```toml
[log]
enable = true
timestamp = true
dirpath = "~/.lssh/log/<Date>/<Hostname>"
remove_ansi_code = false
```

Keepalive and ControlMaster:

```toml
[common]
alive_interval = 10
alive_max = 3
control_master = true
control_path = "/tmp/lssh-control-%h-%p-%r"
control_persist = "10m"
```

OpenSSH config import:

```toml
[sshconfig.default]
path = "~/.ssh/config"
pre_cmd = 'printf "\033]50;SetProfile=local\a"'
post_cmd = 'printf "\033]50;SetProfile=Default\a"'
```

Split config files:

```toml
[includes]
path = [
     "~/.lssh.d/home.conf"
    ,"~/.lssh.d/cloud.conf"
]
```

Local bashrc:

```toml
[server.localrc]
addr = "192.168.100.104"
user = "demo"
key = "~/.ssh/id_rsa"
note = "Use local bashrc files."
local_rc = true
local_rc_compress = true
local_rc_file = [
     "~/dotfiles/.bashrc"
    ,"~/dotfiles/bash_prompt"
    ,"~/dotfiles/sh_alias"
    ,"~/dotfiles/sh_export"
    ,"~/dotfiles/sh_function"
]
```

Per-host pre/post hooks:

```toml
[server.termprofile]
addr = "192.168.100.20"
user = "demo"
key = "~/.ssh/id_rsa"
pre_cmd = 'printf "\033]50;SetProfile=Remote\a"'
post_cmd = 'printf "\033]50;SetProfile=Default\a"'
note = "switch local terminal profile while connected"
```

Port forwarding:

```toml
[server.forward-local]
addr = "192.168.100.30"
user = "demo"
key = "~/.ssh/id_rsa"
port_forward = "local"
port_forward_local = "8080"
port_forward_remote = "localhost:80"

[server.forwards]
addr = "192.168.100.31"
user = "demo"
key = "~/.ssh/id_rsa"
port_forwards = [
    "L:8080:localhost:80",
    "R:80:localhost:8080",
]

[server.dynamic]
addr = "192.168.100.32"
user = "demo"
key = "~/.ssh/id_rsa"
dynamic_port_forward = "10080"
```

Parallel shell (`lsshell`) behavior:

```toml
[shell]
PROMPT = "[${COUNT}] <<< "
OPROMPT = "[${SERVER}][${COUNT}] > "
title = "lsshell"
histfile = "~/.lssh_history"
pre_cmd = "printf 'start lsshell\n'"
post_cmd = "printf 'finish lsshell\n'"

[shell.alias.ll]
command = "ls -lh"

[shell.outexecs.vimdiff]
path = "/usr/bin/vimdiff"
```

HTTP / SOCKS5 / SSH proxy and `ProxyCommand`:

```toml
[proxy.HttpProxy]
addr = "example.com"
port = "8080"

[proxy.Socks5Proxy]
addr = "example.com"
port = "54321"

[server.sshProxyServer]
addr = "192.168.100.200"
key = "/path/to/private_key"
note = "proxy server"

[server.overHttpProxy]
addr = "over-http-proxy.com"
key = "/path/to/private_key"
note = "connect via http proxy"
proxy = "HttpProxy"
proxy_type = "http"

[server.overSocks5Proxy]
addr = "192.168.10.101"
key = "/path/to/private_key"
note = "connect via socks5 proxy"
proxy = "Socks5Proxy"
proxy_type = "socks5"

[server.overProxyServer]
addr = "192.168.10.10"
key = "/path/to/private_key"
note = "connect via ssh proxy"
proxy = "sshProxyServer"

[server.ProxyCommand]
addr = "192.168.10.20"
key = "/path/to/private_key"
note = "connect via ProxyCommand"
proxy_cmd = "ssh -W %h:%p proxy"
```

Authentication patterns:

```toml
[server.PasswordAuth]
addr = "password_auth.local"
user = "user"
pass = "Password"
note = "password auth server"

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

[server.SshAgentAuth]
addr = "agent_auth.local"
user = "user"
agentauth = true
note = "ssh-agent auth server"
```

KnownHosts verification:

```toml
[server.CheckKnownHosts]
addr = "check_known_hosts.local"
user = "user"
check_known_hosts = true
known_hosts_files = ["~/.ssh/known_hosts"]
note = "check known hosts example"
```

Conditional `match` override:

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

## Local bashrc

`lssh` can send your local bash startup files into the remote shell session without leaving those files on the target host.
Use this when you want to reuse your local prompt, aliases, functions, or generated wrappers on remote machines.

```toml
[server.localrc]
addr = "192.168.100.104"
user = "demo"
key = "~/.ssh/id_rsa"
note = "Use local bashrc files."
local_rc = true
local_rc_compress = true
local_rc_file = [
     "~/dotfiles/.bashrc"
    ,"~/dotfiles/bash_prompt"
    ,"~/dotfiles/sh_alias"
    ,"~/dotfiles/sh_export"
    ,"~/dotfiles/sh_function"
]
```

- `local_rc`: enable local rc transfer for the host
- `local_rc_compress`: gzip-compress transferred rc data when the local files are large
- `local_rc_file`: list of local files that are bundled into the remote bash session

This feature is available when the remote login shell is `bash`.
For more details and wrapper examples for tools such as `vim` and `tmux`, see [`cmd/lssh/README.md`](../cmd/lssh/README.md#local-bashrc).

## Per-host `pre_cmd` / `post_cmd`

Use `pre_cmd` and `post_cmd` under `[server.<name>]` when you want to run local commands immediately before connect and after disconnect.
This is useful for changing your local terminal profile, colors, or other local session state while a specific host is active.

```toml
[server.termprofile]
addr = "192.168.100.20"
user = "demo"
key = "~/.ssh/id_rsa"
pre_cmd = 'printf "\033]50;SetProfile=Remote\a"'
post_cmd = 'printf "\033]50;SetProfile=Default\a"'
note = "switch local terminal profile while connected"
```

- `pre_cmd`: local command run before the SSH session starts
- `post_cmd`: local command run after the SSH session ends

These hooks are separate from `[shell].pre_cmd` and `[shell].post_cmd`.
`[server.<name>]` hooks are for normal `lssh` connections to a specific host, while `[shell]` hooks apply to the interactive `lsshell` UI itself.

## Port forwarding

You can define forwarding behavior directly in `[server.<name>]` and then connect with the saved profile.
This covers local and remote port forwarding, dynamic forwarding, HTTP forwarding, NFS forwarding, and X11 forwarding.

Simple local / remote examples:

```toml
[server.forward-local]
addr = "192.168.100.30"
user = "demo"
key = "~/.ssh/id_rsa"
port_forward = "local"
port_forward_local = "8080"
port_forward_remote = "localhost:80"

[server.forward-remote]
addr = "192.168.100.31"
user = "demo"
key = "~/.ssh/id_rsa"
port_forward = "remote"
port_forward_local = "80"
port_forward_remote = "localhost:8080"
```

Multiple forwards and other forwarding modes:

```toml
[server.forwards]
addr = "192.168.100.40"
user = "demo"
key = "~/.ssh/id_rsa"
port_forwards = [
    "L:8080:localhost:80",
    "R:80:localhost:8080",
]

[server.dynamic]
addr = "192.168.100.41"
user = "demo"
key = "~/.ssh/id_rsa"
dynamic_port_forward = "10080"

[server.reverse-dynamic]
addr = "192.168.100.42"
user = "demo"
key = "~/.ssh/id_rsa"
reverse_dynamic_port_forward = "10080"

[server.http-dynamic]
addr = "192.168.100.43"
user = "demo"
key = "~/.ssh/id_rsa"
http_dynamic_port_forward = "18080"

[server.http-reverse-dynamic]
addr = "192.168.100.44"
user = "demo"
key = "~/.ssh/id_rsa"
http_reverse_dynamic_port_forward = "18080"

[server.nfs-dynamic]
addr = "192.168.100.45"
user = "demo"
key = "~/.ssh/id_rsa"
nfs_dynamic_forward = "2049"
nfs_dynamic_forward_path = "/path/to/remote"

[server.nfs-reverse-dynamic]
addr = "192.168.100.46"
user = "demo"
key = "~/.ssh/id_rsa"
nfs_reverse_dynamic_forward = "2049"
nfs_reverse_dynamic_forward_path = "/path/to/local"

[server.x11]
addr = "192.168.100.47"
user = "demo"
key = "~/.ssh/id_rsa"
x11 = true

[server.x11-trusted]
addr = "192.168.100.48"
user = "demo"
key = "~/.ssh/id_rsa"
x11_trusted = true
```

Available forwarding settings:

- `port_forward`: single forward mode. Use `local` or `remote`
- `port_forward_local`: local side of a single forward, for example `8080` or `127.0.0.1:8080`
- `port_forward_remote`: remote side of a single forward, for example `localhost:80`
- `port_forwards`: multiple forwards in compact form such as `L:8080:localhost:80`
- `dynamic_port_forward`: local SOCKS5 forward port
- `reverse_dynamic_port_forward`: reverse dynamic forward port
- `http_dynamic_port_forward`: local HTTP proxy forward port
- `http_reverse_dynamic_port_forward`: reverse HTTP proxy forward port
- `nfs_dynamic_forward`: local port used for NFS dynamic forwarding
- `nfs_dynamic_forward_path`: remote path exported through NFS dynamic forwarding
- `nfs_reverse_dynamic_forward`: local port used for reverse NFS forwarding
- `nfs_reverse_dynamic_forward_path`: local path exported for reverse NFS forwarding
- `x11`: enable X11 forwarding
- `x11_trusted`: enable trusted X11 forwarding

Tunnel device forwarding is available only from the command line with `--tunnel`, not as a config file key.


## Terminal log

You can record interactive terminal output to local log files with the `[log]` section.
These settings are shared by commands that open remote terminals, including `lssh`, `lsshell`, and mux-based views(`lssh -P`, `lsmux`).

```toml
[log]
enable = true
timestamp = true
dirpath = "~/.lssh/log/<Date>/<Hostname>"
remove_ansi_code = false
```

- `enable`: turn terminal logging on or off
- `timestamp`: prepend each logged line with a local timestamp like `2006/01/02 15:04:05`
- `dirpath`: directory where log files are created
- `remove_ansi_code`: strip ANSI escape sequences before writing the log

Log files are created as `YYYYmmdd_HHMMSS_servername.log`.
In `dirpath`, `~` expands to your home directory, `<Date>` becomes the current date in `YYYYmmdd` format, and `<Hostname>` becomes the selected server name.

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

If `~/.lssh.conf` does not exist, the `lssh` suite commands show an information
message and fall back to OpenSSH config import mode automatically. In an
interactive terminal, they can also offer to create `~/.lssh.conf` from
`~/.ssh/config` on the spot.

You can also specify and read another path.
In addition to the path, server config items can be specified and applied collectively.

```toml
[sshconfig.default]
path = "~/.ssh/config"
pre_cmd = 'printf "\033]50;SetProfile=local\a"'
post_cmd = 'printf "\033]50;SetProfile=Default\a"'
```

You can also generate a starter `~/.lssh.conf` from any suite command and write
it with shell redirection:

```bash
lssh --generate-lssh-conf > ~/.lssh.conf
lssh --generate-lssh-conf=~/.ssh/config.work > ~/.lssh.conf
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

## Parallel Interactive shell settings with `[shell]`

Use `[shell]` to customize the interactive behavior of `lsshell`.
These settings control the prompt text, shell title, history file, startup and shutdown hooks, command aliases, and local helper commands started by `%outexec`.

```toml
[shell]
PROMPT = "[${COUNT}] <<< "
OPROMPT = "[${SERVER}][${COUNT}] > "
title = "lsshell"
histfile = "~/.lssh_history"
pre_cmd = "printf 'start lsshell\n'"
post_cmd = "printf 'finish lsshell\n'"

[shell.alias.ll]
command = "ls -lh"

[shell.outexecs.vimdiff]
path = "/usr/bin/vimdiff"
```

Available `shell` settings:

- `PROMPT`: prompt shown before you enter a command
- `OPROMPT`: prefix used when displaying command output from each host
- `title`: title text shown by the shell UI
- `histfile`: path to the history file used by `lsshell`
- `pre_cmd`: local command run before starting the interactive shell
- `post_cmd`: local command run after the shell exits
- `[shell.alias.<name>]`: define reusable aliases inside `lsshell`
- `[shell.outexecs.<name>]`: define local commands callable through `%outexec`

## Multiplexer settings with `[mux]`

Use `[mux]` to customize `lsmux` key bindings and pane colors.
This section affects only the multiplexer UI. Host connection settings such as `addr`, `user`, `key`, and proxy settings remain under `[common]` and `[server.<name>]`.

```toml
[mux]
prefix = "Ctrl+A"
quit = "&"
new_page = "c"
new_pane = "s"
split_horizontal = "\""
split_vertical = "%"
next_pane = "o"
next_page = "n"
prev_page = "p"
page_list = "w"
close_pane = "x"
broadcast = "b"
transfer = "f"
focus_border_color = "green"
focus_title_color = "green"
broadcast_border_color = "yellow"
broadcast_title_color = "yellow"
done_border_color = "gray"
done_title_color = "gray"
```

Available `mux` settings:

- `prefix`: prefix key used before `lsmux` subcommands. Default: `Ctrl+A`
- `quit`: quit `lsmux`. Default: `&`
- `new_page`: create a new page. Default: `c`
- `new_pane`: open a host selector and add a pane. Default: `s`
- `split_horizontal`: split the current pane horizontally. Default: `"`
- `split_vertical`: split the current pane vertically. Default: `%`
- `next_pane`: move focus to the next pane. Default: `o`
- `next_page`: switch to the next page. Default: `n`
- `prev_page`: switch to the previous page. Default: `p`
- `page_list`: show the page list. Default: `w`
- `close_pane`: close the current pane. Default: `x`
- `broadcast`: toggle broadcast input to all panes on the page. Default: `b`
- `transfer`: open file transfer for the active pane. Default: `f`
- `focus_border_color`, `focus_title_color`: colors for the focused pane. Default: `green`
- `broadcast_border_color`, `broadcast_title_color`: colors for panes in broadcast mode. Default: `yellow`
- `done_border_color`, `done_title_color`: colors for completed command panes. Default: `gray`
