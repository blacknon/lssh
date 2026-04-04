lscp
===

## About

`lscp` is an SCP client that lets you select hosts from the configuration file and copy files over SSH.
It supports local-to-remote, remote-to-local, and remote-to-remote transfers.
Although the command interface is SCP-style, file transfers are performed using the SFTP protocol over SSH.

## Usage

```shell
$ lscp --help
NAME:
    lscp - TUI list select and parallel scp client command.
USAGE:
    lscp [options] (local|remote):from_path... (local|remote):to_path

OPTIONS:
    --host servername, -H servername  connect servernames
    --list, -l                        print server list from config
    --file filepath, -F filepath      config file path (default: "/Users/blacknon/.lssh.conf")
    --parallel value, -P value        parallel file copy count per host (default: 1)
    --permission, -p                  copy file permission
    --help, -h                        print this help
    --version, -v                     print the version

COPYRIGHT:
    blacknon(blacknon@orebibou.com)

VERSION:
    lssh-suite 0.7.0 (stable/core)

USAGE:
    # local to remote scp
    lscp /path/to/local... remote:/path/to/remote

    # remote to local scp
    lscp remote:/path/to/remote... /path/to/local

    # remote to remote scp
    lscp remote:/path/to/remote... remote:/path/to/local
```

## OverView

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

You can select the destination host from the TUI list, or specify it directly with `-H`.
When both source and destination are remote paths, `lscp` first asks for the source host and then for the destination host.

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
The host name part must match a server name defined in `~/.lssh.conf`.

You cannot mix local and remote paths in the source arguments.
For example, all source paths must be local paths or all source paths must be remote paths.
