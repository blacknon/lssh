[![Go Report Card](https://goreportcard.com/badge/github.com/blacknon/lssh)](https://goreportcard.com/report/github.com/blacknon/lssh)

lssh
====

<p align="center">
  <img src="./images/lssh_macosx.gif" width="33%" />
  <img src="./images/lssh_linux.gif" width="33%" />
  <img src="./images/lssh_windows.gif" width="33%" />
</p>

`lssh` is a TUI-first SSH client for operators who work across multiple servers.

Choose hosts from an interactive selector, connect immediately, run commands in parallel, reuse your local bashrc on remote shells without leaving files behind, and use advanced forwarding including NFS-based mounts.

## Why start with `lssh`

### Pick servers from a TUI, then connect or run in parallel

`lssh` reads your server inventory from TOML and opens a TUI selector when you do not specify `-H`.
You can filter by typing, select one or more hosts, and then either open an interactive SSH session or run the same command on all selected hosts.

It is especially useful when your visible target list should change depending on where you are running from.
With conditional `match` rules, you can show, hide, or override hosts by local network, OS, terminal, environment variables, and more.

Examples:

```bash
# open the selector, then connect to one host
lssh

# open the selector, then run a command on the selected host
lssh hostname

# select multiple hosts and run the same command in parallel
lssh -p uname -a
```

### Use your local bashrc without leaving files on the remote host

`lssh` can send your local shell startup files such as `.bashrc`, aliases, helper functions, or generated wrappers into the remote shell session without permanently placing those files on the server.

That means you can keep using your local workflow on SSH targets while avoiding configuration drift on the remote side.
This is handy when you want your prompt, aliases, helper commands, or even wrappers for tools like `vim` and `tmux`, but you do not want to "pollute" each server with personal dotfiles.

For the detailed setup, see [`local bashrc`](./cmd/lssh/README.md#local-bashrc).

### Mount your local directory on a remote server

`lssh` can expose a local directory to a remote server over NFS reverse forwarding, so you can use local files and tools in remote workflows without copying them onto the host.

Beyond interactive SSH login, `lssh` also supports:

- NFS reverse forwarding for mounting a local directory on a remote server
- SSH local / remote port forwarding
- SOCKS5 and HTTP dynamic forwarding
- X11 forwarding
- Multi-stage proxy routes over SSH, HTTP, SOCKS5, and `ProxyCommand`

For examples, see [`forwarding`](./cmd/lssh/README.md#forwarding) and the shared configuration docs in [`docs/`](./docs/README.md).

## Try it quickly

### 1. Install

Use whichever path is easiest for you:

```bash
brew install blacknon/lssh/lssh
```

```bash
go install github.com/blacknon/lssh/cmd/lssh@latest
```

Prebuilt packages and the full suite are also available on GitHub Releases.
See the install details in [`docs/install.md`](./docs/install.md).

### 2. Create a minimal config

Create `~/.lssh.conf`:

```toml
[common]
user = "demo"
key = "~/.ssh/id_rsa"

[server.dev]
addr = "192.168.100.10"
note = "development"

[server.stg]
addr = "192.168.100.20"
note = "staging"
```

### 3. Start with these commands

```bash
# choose from the TUI and open a shell
lssh

# choose from the TUI and run a command
lssh hostname

# choose multiple hosts and run in parallel
lssh -p 'uptime'
```

If you want a ready-to-run local playground, see [`demo/README.md`](./demo/README.md).

## What else is in the suite

You can stay on `lssh` for most SSH access workflows, but the repository also includes other tools when the job is more specialized.

| Command | Best for | README |
| --- | --- | --- |
| `lssh` | Interactive SSH, selection TUI, parallel commands, forwarding | [cmd/lssh/README.md](./cmd/lssh/README.md) |
| `lscp` | SCP-style file copy | [cmd/lscp/README.md](./cmd/lscp/README.md) |
| `lsftp` | Interactive file transfer and remote file operations | [cmd/lsftp/README.md](./cmd/lsftp/README.md) |
| `lssync` | One-way sync over SSH/SFTP | [cmd/lssync/README.md](./cmd/lssync/README.md) |
| `lsshell` | Broadcast-style multi-host shell | [cmd/lsshell/README.md](./cmd/lsshell/README.md) |
| `lsmux` | Pane-based multi-host SSH workspace | [cmd/lsmux/README.md](./cmd/lsmux/README.md) |
| `lsmon` | Multi-host monitoring UI | [cmd/lsmon/README.md](./cmd/lsmon/README.md) |

If all you need is SSH access, start with `lssh`.
When you later need file transfer, sync, monitoring, or a pane UI, the rest of the suite is there.

## Docs

- [docs/README.md](./docs/README.md): documentation index
- [cmd/lssh/README.md](./cmd/lssh/README.md): `lssh` command details, forwarding, and local rc usage
- [cmd/README.md](./cmd/README.md): command overview

## Related projects

- [go-sshlib](https://github.com/blacknon/go-sshlib): Go library for SSH connections, command execution, and interactive shells

## Licence

[MIT](LICENSE.md)

## Author

[blacknon](https://github.com/blacknon)
