lssync
===

## About

`lssync` is a one-way sync command built on the same SSH/SFTP stack as `lscp`.
It synchronizes a source tree to a destination tree without requiring `rsync` on the remote host.
The command keeps the `lscp` style CLI and host-selection flow, but applies source-of-truth semantics and optionally deletes extra destination entries with `--delete`.

## Usage

```shell
$ lssync --help
NAME:
    lssync - TUI list select and parallel one-way sync command over SSH/SFTP.
USAGE:
    lssync [options] (local|remote):from_path... (local|remote):to_path

OPTIONS:
    --host value, -H value      connect servernames
    --list, -l                  print server list from config
    --file value, -F value      config file path (default: "/Users/blacknon/.lssh.conf")
    --parallel value, -P value  parallel file sync count per host (default: 1)
    --permission, -p            copy file permission
    --delete                    delete destination entries that do not exist in source
    --help, -h                  print this help
    --version, -v               print the version

COPYRIGHT:
    blacknon(blacknon@orebibou.com)

VERSION:
    lssh-suite 0.8.0 (beta/transfer)

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

The source side is treated as the source of truth.
Entries missing from the destination are created, changed files are updated, and destination-only entries are deleted only when `--delete` is specified.

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
```

## Notes

- Like `lscp`, transfers are implemented with SFTP over SSH.
- `--delete` only removes entries inside the destination scope derived from the source roots.
- For multiple sources, the destination is treated as a directory and the union of source entries becomes the desired state.
