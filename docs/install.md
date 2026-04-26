Install
=======

You can install only `lssh`, or the full tool suite.

## Prebuilt binaries

Prebuilt binaries are available on GitHub Releases.

### Linux (amd64, tar.gz)

```bash
VERSION=0.10.0
curl -fL -o /tmp/lssh.tar.gz \
  "https://github.com/blacknon/lssh/releases/download/v${VERSION}/lssh_${VERSION}_linux_amd64.tar.gz"
sudo tar -xzf /tmp/lssh.tar.gz -C /tmp
sudo install -m 0755 /tmp/lssh_${VERSION}_linux_amd64/bin/* /usr/local/bin/
```

### Debian / Ubuntu (.deb)

```bash
VERSION=0.10.0
curl -fL -o /tmp/lssh.deb \
  "https://github.com/blacknon/lssh/releases/download/v${VERSION}/lssh_${VERSION}_amd64.deb"
sudo apt install /tmp/lssh.deb
```

### RHEL / Fedora / Rocky / AlmaLinux (.rpm)

```bash
VERSION=0.10.0
curl -fL -o /tmp/lssh.rpm \
  "https://github.com/blacknon/lssh/releases/download/v${VERSION}/lssh-${VERSION}-1.x86_64.rpm"
sudo dnf install -y /tmp/lssh.rpm
```

## Package layout

`lssh` provides both a full suite package and smaller split packages.

| Package | Includes | Best for |
| --- | --- | --- |
| `lssh_*` | `lssh`, `lscp`, `lsftp`, `lssync`, `lsdiff`, `lsshfs`, `lsmon`, `lsshell`, `lsmux`, `lspipe` | Full installation of the entire tool suite |
| `lssh-core_*` | `lssh` | SSH access and forwarding only |
| `lssh-transfer_*` | `lscp`, `lsftp`, `lssync`, `lsdiff`, `lsshfs` | File transfer, diff, and mount workflows only |
| `lssh-monitor_*` | `lsmon` | Monitoring multiple remote hosts |
| `lssh-sysadmin_*` | `lsshell`, `lsmux`, `lspipe` | Parallel shell and multi-host operations |

## go install

Install commands directly with Go:

```bash
go install github.com/blacknon/lssh/cmd/lssh@latest
go install github.com/blacknon/lssh/cmd/lscp@latest
go install github.com/blacknon/lssh/cmd/lsftp@latest
go install github.com/blacknon/lssh/cmd/lssync@latest
go install github.com/blacknon/lssh/cmd/lsdiff@latest
go install github.com/blacknon/lssh/cmd/lsshfs@latest
go install github.com/blacknon/lssh/cmd/lsshell@latest
go install github.com/blacknon/lssh/cmd/lsmon@latest
go install github.com/blacknon/lssh/cmd/lsmux@latest
go install github.com/blacknon/lssh/cmd/lspipe@latest
```

### Provider binaries

Provider-backed inventory, secret, and connector features require provider executables in addition to the main `cmd/*` binaries.
If you install only `cmd/lssh`, provider binaries are not installed automatically.

For local development, install the bundled provider executables with:

```bash
mise run provider_install
```

This installs the provider binaries under `~/.config/lssh/providers`.
The same task first builds the current provider set from this repository, including:

- `provider-mixed-aws-ec2`
- `provider-connector-openssh`
- `provider-connector-telnet`
- `provider-connector-winrm`
- `provider-mixed-azure-compute`
- `provider-mixed-gcp-compute`
- `provider-inventory-proxmox`
- `provider-secret-onepassword`
- `provider-secret-bitwarden`
- `provider-secret-os-keychain`
- `provider-secret-custom-script`

If you are using a release build instead of a source checkout, install the matching provider binaries from the release assets or the full suite package.

## Homebrew

```bash
brew install blacknon/lssh/lssh
```

## Requirements

- Build from source with Go `1.25.1` or newer, matching [go.mod](../go.mod).
- `lsshfs` uses a different local mount backend on each OS:
- Linux: FUSE support and a working `fusermount`/FUSE setup are required.
- macOS: `mount_nfs` is used locally, so the client must allow local NFS mounts.
- Windows: `lsshfs` is currently not supported.
- The repository intentionally replaces `github.com/kevinburke/ssh_config` with the vendored fork at `./internal/ssh_config` so the generated config and parser behavior stay in sync with `lssh`.

## Shell completion

Install completion files for `bash`, `zsh`, and `fish`:

```bash
make install-completions-user
```

To install under `/usr/local` instead:

```bash
sudo make install-completions
```

You can also install just one shell:

```bash
make install-completions-user COMPLETION_SHELL=zsh
make install-completions-user COMPLETION_SHELL=bash
make install-completions-user COMPLETION_SHELL=fish
```

If you use `mise`, the same flow is available as:

```bash
mise run completion_install
mise run completion_install_system
```

For user-level `zsh` installs, add this to `~/.zshrc` if needed:

```bash
fpath=($HOME/.zfunc $fpath)
autoload -Uz compinit && compinit
```

## Build from source

```bash
git clone https://github.com/blacknon/lssh.git
cd lssh
make build
sudo make install
```
