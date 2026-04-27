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

### Windows (amd64, .zip)

Download a release asset from GitHub Releases, then extract it somewhere under your user profile.

- `lssh_${VERSION}_windows_amd64.zip`: full command suite
- `lssh-core_${VERSION}_windows_amd64.zip`: `lssh` only
- `lssh-complete_${VERSION}_windows_amd64.zip`: full suite plus bundled providers

There is currently no MSI installer.
After extracting the archive, place the files from `bin\` in a directory that is included in your `PATH`.

If you use provider-backed inventory, connector, or secret workflows, also install the matching provider executables from `providers\`.
For package manager publishing guidance such as `winget`, `Scoop`, `Homebrew`, and `AUR`, see [distribution.md](./distribution.md).

## Package layout

`lssh` provides both a full suite package and smaller split packages.

| Package | Includes | Best for |
| --- | --- | --- |
| `lssh-complete_*` | all suite commands, bundled providers, and command completions | A single archive with the full suite plus provider-backed workflows |
| `lssh_*` | `lssh`, `lscp`, `lsftp`, `lssync`, `lsdiff`, `lsshfs`, `lsmon`, `lsshell`, `lsmux`, `lspipe` | Full installation of the entire tool suite |
| `lssh-core_*` | `lssh` | SSH access and forwarding only |
| `lssh-transfer_*` | `lscp`, `lsftp`, `lssync`, `lsdiff`, `lsshfs` | File transfer, diff, and mount workflows only |
| `lssh-monitor_*` | `lsmon` | Monitoring multiple remote hosts |
| `lssh-sysadmin_*` | `lsshell`, `lsmux`, `lspipe` | Parallel shell and multi-host operations |
| `lssh-providers_*` | bundled provider executables | Provider-backed inventory, connector, and secret workflows |

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

In `v0.10.0`, provider-backed inventory and secret workflows are best treated as `beta`.
Connector-backed access beyond native SSH is still `experimental`, especially for non-SSH runtimes such as WinRM and cloud-managed connectors.

For local development, install the bundled provider executables with:

```bash
mise run provider_install
```

If you want the local source checkout to install the full suite, bundled providers, and shell completions in one step, use:

```bash
mise run complete_install
```

This installs the provider binaries under `~/.config/lssh/providers`.
The same task first builds the current provider set from this repository, including:

- `provider-mixed-aws-ec2`
- `provider-connector-openssh`
- `provider-connector-telnet`
- `provider-connector-winrm`
- `provider-mixed-azure-compute`
- `provider-inventory-azure-compute`
- `provider-mixed-gcp-compute`
- `provider-inventory-gcp-compute`
- `provider-inventory-proxmox`
- `provider-secret-onepassword`
- `provider-secret-bitwarden`
- `provider-secret-custom-script`

On macOS, the same release/install flow also includes `provider-secret-os-keychain`.

If you are using a release build instead of a source checkout, either:

- install the all-in-one `lssh-complete_*` archive for your platform, or
- install the matching `lssh-providers_*` release asset alongside the command package you want to use

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
