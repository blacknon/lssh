lsshfs
======

## About

`lsshfs` mounts a remote directory from a single host in your `lssh` inventory.
It reuses the same host selection and SSH config flow as `lssh`, but exposes the remote path through a local mount backend that depends on the client OS.

- Linux clients use `FUSE`
- macOS clients use `NFS`
- Windows clients use `NFS` via the built-in Windows Client for NFS

The command runs in the background by default and automatically unmounts when the SSH connection is lost.

## Usage

```shell
$ lsshfs --help
NAME:
    lsshfs - Single-host SSH mount command with OS-specific local mount backends.
USAGE:
    lsshfs [options] [host:]remote_path mountpoint

OPTIONS:
    --host servername, -H servername    connect servername.
    --file filepath, -F filepath        config filepath. (default: "/Users/blacknon/.lssh.conf")
    --generate-lssh-conf ~/.ssh/config  print generated lssh config from OpenSSH config to stdout (~/.ssh/config by default).
    --rw                                mount as read-write (current default behavior).
    --unmount                           unmount the specified mountpoint and stop the background process.
    --list-mounts                       list active lsshfs mount records.
    --foreground                        run in the foreground for debugging and tests.
    --list, -l                          print server list from config.
    --help, -h                          print this help
    --enable-control-master             temporarily enable ControlMaster for this command execution
    --disable-control-master            temporarily disable ControlMaster for this command execution
    --version, -v                       print the version

VERSION:
    lssh-suite 0.9.0 (beta/transfer)

USAGE:
    # mount a remote path from the selected host
    lsshfs /srv/data ~/mnt/data

    # mount a remote path from the named inventory host
    lsshfs @app:/srv/data ~/mnt/data

    # unmount
    lsshfs --unmount ~/mnt/data

    # windows example
    lsshfs @app:/srv/data Z:

```

## Overview

### mount a remote path locally

`lsshfs` resolves exactly one target host, then mounts one remote directory to one local mountpoint.
You can specify the host explicitly with `@host:/path`, use `-H`, or let the TUI picker choose the host for you.

```bash
lsshfs @web01:/var/www ./mnt/web01
lsshfs -H web01 /var/www ./mnt/web01
lsshfs --rw @web01:/srv/app ./mnt/app
```

### inspect and unmount mounts

Mounts are recorded locally so you can list them later or unmount by mountpoint.
This is the easiest way to clean up background sessions after a long-lived mount.

```bash
lsshfs --unmount ./mnt/app
lsshfs --list-mounts
```

### windows mount targets

Windows uses a drive letter as the mount target:

```powershell
lsshfs @web01:/srv/data Z:
```

### notes

- `lsshfs` supports only one host at a time.
- `@host:/path` is the preferred remote path format, but `host:/path` is still accepted for compatibility.
- On macOS, the local mount is created with `mount_nfs`.
- On Windows, the local mount is created with the built-in `mount` / `umount` commands for Client for NFS.
- On Windows, Client for NFS must be enabled ahead of time or `lsshfs` returns an explicit error.
- On Windows, the local NFS server listens on port `2049`, so binding that port must be allowed in the environment.
- The default config search order is `~/.lssh.toml`, `~/.lssh.yaml`, `~/.lssh.yml`, then `~/.lssh.conf`.
