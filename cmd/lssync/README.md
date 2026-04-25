lssync
===

## About

`lssync` is a sync command built on the same SSH/SFTP stack as `lscp`.
It synchronizes a source tree to a destination tree without requiring `rsync` on the remote host.
The command keeps the `lscp` style CLI and host-selection flow.
Use `-B` to switch from one-way sync to bidirectional sync, and use `-D` to run either mode continuously as a daemon.

## Usage

```shell
$ lssync --help
NAME:
    lssync - TUI list select and parallel sync command over SSH/SFTP.
USAGE:
    lssync [options] (local|remote):from_path... (local|remote):to_path

OPTIONS:
    --host value, -H value              connect servernames
    --list, -l                          print server list from config
    --file value, -F value              config file path (default: "/Users/blacknon/.lssh.conf")
    --generate-lssh-conf ~/.ssh/config  print generated lssh config from OpenSSH config to stdout (~/.ssh/config by default).
    --daemon, -D                        run as a daemon and repeat sync at each interval
    --daemon-interval value             daemon sync interval (default: 5s)
    --bidirectional, -B                 sync both sides and copy newer changes in either direction
    --parallel value, -P value          parallel file sync count per host (default: 1)
    --permission, -p                    copy file permission
    --dry-run                           show sync actions without modifying files
    --delete                            delete destination entries that do not exist in source
    --help, -h                          print this help
    --enable-control-master             temporarily enable ControlMaster for this command execution
    --disable-control-master            temporarily disable ControlMaster for this command execution
    --version, -v                       print the version

COPYRIGHT:
    blacknon(blacknon@orebibou.com)

VERSION:
    lssh-suite 0.9.1 (beta/transfer)

USAGE:
    # local to remote sync
    lssync /path/to/local... remote:/path/to/remote

    # remote to local sync
    lssync remote:/path/to/remote... /path/to/local

    # remote to remote sync
    lssync remote:/path/to/remote... remote:/path/to/local
```

## Overview

`lssync` supports the same path syntax and host selection patterns as `lscp`.
Supported flows are:

- Local to remote sync
- Remote to local sync
- Remote to remote sync
- Daemon mode with `-D`
- Bidirectional mode with `-B`

In one-way mode, the source side is treated as the source of truth.
Entries missing from the destination are created, changed files are updated, and destination-only entries are deleted only when `--delete` is specified.

In bidirectional mode, both sides are scanned and the newer side wins when the same file exists on both sides with different timestamps.
With `-D -B`, bidirectional sync repeats at each daemon interval and processes selected hosts sequentially in each cycle.

## Examples

```bash
# local -> remote
lssync ./dist remote:/srv/app

# remote -> local
lssync remote:/var/lib/app ./backup

# remote -> remote
lssync remote:/srv/app remote:/srv/app

# sync with delete
lssync --delete ./site remote:/var/www/site

# one-way daemon sync
lssync -D --daemon-interval 10s ./site remote:/var/www/site

# bidirectional sync
lssync -B ./notes remote:/srv/notes

# bidirectional daemon sync
lssync -D -B --daemon-interval 30s ./notes remote:/srv/notes
```

## Notes

- Like `lscp`, transfers are implemented with SFTP over SSH.
- Connector-backed targets that do not advertise `sftp_transport` are excluded from the selection list and rejected when specified explicitly.
- `--delete` only removes entries inside the destination scope derived from the source roots.
- For multiple sources, the destination is treated as a directory and the union of source entries becomes the desired state.
- Bidirectional sync currently requires exactly one source path and one destination path.
- `--delete` is not supported together with bidirectional sync.
- When multiple hosts are selected in bidirectional mode, hosts are processed sequentially for safer conflict handling.
- The default config search order is `~/.lssh.toml`, `~/.lssh.yaml`, `~/.lssh.yml`, then `~/.lssh.conf`.
