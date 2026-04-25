lscp
===

<p align="center">
<img src="./img/lscp.gif" width="720" />
</p>

## About

`lscp` is an SCP client that lets you select hosts from the configuration file and copy files over SSH.
It supports local-to-remote, remote-to-local, and remote-to-remote transfers.
Although the command interface is SCP-style, file transfers are performed using the SFTP protocol over SSH.

Release packages may use the split-package naming `lssh-scp`, but the command you run remains `lscp`.

## Usage

```shell
$ lscp --help
NAME:
    lscp - TUI list select and parallel scp client command.
USAGE:
    lscp [options] (local|remote):from_path... (local|remote):to_path

OPTIONS:
    --host value, -H value              connect servernames
    --list, -l                          print server list from config
    --file value, -F value              config file path (default: "/Users/blacknon/.lssh.conf")
    --generate-lssh-conf ~/.ssh/config  print generated lssh config from OpenSSH config to stdout (~/.ssh/config by default).
    --parallel value, -P value          parallel file copy count per host (default: 1)
    --permission, -p                    copy file permission
    --dry-run                           show copy actions without modifying files
    --help, -h                          print this help
    --enable-control-master             temporarily enable ControlMaster for this command execution
    --disable-control-master            temporarily disable ControlMaster for this command execution
    --version, -v                       print the version

COPYRIGHT:
    blacknon(blacknon@orebibou.com)

VERSION:
    lssh-suite 0.9.1 (stable/transfer)

USAGE:
    # local to remote scp
    lscp /path/to/local... remote:/path/to/remote

    # remote to local scp
    lscp remote:/path/to/remote... /path/to/local

    # remote to remote scp
    lscp remote:/path/to/remote... remote:/path/to/local

```

## Overview

### transfer types

The following transfer patterns are available

- Local to remote copy
- Remote to local copy
- Remote to remote copy

Command line examples.

```bash
# local -> remote
lscp ./local.txt remote:/tmp/

# remote -> local
lscp remote:/var/log/app.log ./

# remote -> remote
lscp remote:/var/log/app.log remote:/tmp/
```

### host selection

You can also copy files with `get` or `put` across multiple hosts at once.
In that case, select multiple hosts in the host selection screen.

```bash
# put the same file to multiple hosts selected in the TUI
lscp ./build/app remote:/opt/app/
```

You can select the destination host from the TUI list, or specify it directly with `-H`.
When both source and destination are remote paths, `lscp` first asks for the source host and then for the destination host.
Connector-backed targets that do not advertise `sftp_transport` are excluded from the selection list and rejected when passed with `-H`.

```bash
# specify destination hosts directly
lscp -H web01 ./build/app remote:/opt/app/
```

### parallel copy

You can increase the number of concurrent copies per host with `-P`.
This is useful when sending many files to the same host.

```bash
# increase parallel copy count per host
lscp -P 4 ./dist/* remote:/srv/releases/
```

### path rules

Remote paths use the `name:/path/to/file` format.
The host name part must match a server name defined in your lssh config file.

You cannot mix local and remote paths in the source arguments.
For example, all source paths must be local paths or all source paths must be remote paths.

The default config search order is `~/.lssh.toml`, `~/.lssh.yaml`, `~/.lssh.yml`, then `~/.lssh.conf`.
