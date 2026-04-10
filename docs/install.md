Install
=======

You can install only `lssh`, or the full tool suite.

## Prebuilt binaries

Prebuilt binaries are available on GitHub Releases.

### Linux (amd64, tar.gz)

```bash
VERSION=0.8.0
curl -fL -o /tmp/lssh.tar.gz \
  "https://github.com/blacknon/lssh/releases/download/v${VERSION}/lssh_${VERSION}_linux_amd64.tar.gz"
sudo tar -xzf /tmp/lssh.tar.gz -C /tmp
sudo install -m 0755 /tmp/lssh_${VERSION}_linux_amd64/bin/* /usr/local/bin/
```

### Debian / Ubuntu (.deb)

```bash
VERSION=0.8.0
curl -fL -o /tmp/lssh.deb \
  "https://github.com/blacknon/lssh/releases/download/v${VERSION}/lssh_${VERSION}_amd64.deb"
sudo apt install /tmp/lssh.deb
```

### RHEL / Fedora / Rocky / AlmaLinux (.rpm)

```bash
VERSION=0.8.0
curl -fL -o /tmp/lssh.rpm \
  "https://github.com/blacknon/lssh/releases/download/v${VERSION}/lssh-${VERSION}-1.x86_64.rpm"
sudo dnf install -y /tmp/lssh.rpm
```

## Package layout

`lssh` provides both a full suite package and smaller split packages.

| Package | Includes | Best for |
| --- | --- | --- |
| `lssh_*` | `lssh`, `lscp`, `lsftp`, `lssync`, `lsmon`, `lsshell`, `lsmux` | Full installation of the entire tool suite |
| `lssh-core_*` | `lssh` | SSH access and forwarding only |
| `lssh-transfer_*` | `lscp`, `lsftp`, `lssync` | File transfer workflows only |
| `lssh-monitor_*` | `lsmon` | Monitoring multiple remote hosts |
| `lssh-sysadmin_*` | `lsshell`, `lsmux` | Parallel shell and multi-host operations |

## go install

Install commands directly with Go:

```bash
go install github.com/blacknon/lssh/cmd/lssh@latest
go install github.com/blacknon/lssh/cmd/lscp@latest
go install github.com/blacknon/lssh/cmd/lsftp@latest
go install github.com/blacknon/lssh/cmd/lssync@latest
go install github.com/blacknon/lssh/cmd/lsshell@latest
go install github.com/blacknon/lssh/cmd/lsmon@latest
go install github.com/blacknon/lssh/cmd/lsmux@latest
```

## Homebrew

```bash
brew install blacknon/lssh/lssh
```

## Shell completion

Install completion files for `bash`, `zsh`, and `fish`:

```bash
./scripts/install-completions.sh all --user
```

To install under `/usr/local` instead:

```bash
sudo ./scripts/install-completions.sh all --system
```

You can also install just one shell:

```bash
./scripts/install-completions.sh zsh --user
./scripts/install-completions.sh bash --user
./scripts/install-completions.sh fish --user
```

If you use `mise`, the same flow is available as:

```bash
mise run completion_install
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
