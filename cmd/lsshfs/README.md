lsshfs
======

## About

`lsshfs` mounts a remote directory from a single host in your `lssh` inventory.
It reuses the same host selection and SSH config flow as `lssh`, but exposes the remote path through a local mount backend that depends on the client OS.

- Linux clients use `FUSE`
- macOS clients use `NFS`
- Windows is currently not supported
- connector-backed mounts that rely on `sftp_transport` are currently supported on Linux and macOS

The command runs in the background by default and automatically unmounts when the SSH connection is lost.
If a normal unmount fails, `lsshfs` also tries stronger fallback commands to reduce stale mounts and Finder impact on macOS.

## Usage

```shell
$ lsshfs --help
NAME:
    lsshfs - Single-host SSH mount command with FUSE/NFS local mount backends.
USAGE:
    lsshfs [options] [host:]remote_path mountpoint

OPTIONS:
    --host servername, -H servername    connect servername.
    --file filepath, -F filepath        config filepath. (default: "/Users/blacknon/.lssh.conf")
    --generate-lssh-conf ~/.ssh/config  print generated lssh config from OpenSSH config to stdout (~/.ssh/config by default).
    --mount-option value                append local mount option (repeatable).
    --debug                             enable debug logging for lsshfs and go-sshlib.
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
    lssh-suite 0.10.0 (beta/transfer)

USAGE:
    # mount a remote path from the selected host
    lsshfs /srv/data ~/mnt/data

    # mount a remote path from the named inventory host
    lsshfs @app:/srv/data ~/mnt/data

    # unmount
    lsshfs --unmount ~/mnt/data

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

### notes

- `lsshfs` supports only one host at a time.
- `@host:/path` is the preferred remote path format, but `host:/path` is still accepted for compatibility.
- On macOS, the local mount is created with `mount_nfs`.
- On Windows, `lsshfs` is currently not supported.
- The default config search order is `~/.lssh.toml`, `~/.lssh.yaml`, `~/.lssh.yml`, then `~/.lssh.conf`.

### mount options

You can append local mount options from config or CLI. This is mainly useful on macOS when you want to tune `mount_nfs` behavior without changing the backend.

```toml
[lsshfs]
mount_options = ["nobrowse"]

[lsshfs.darwin]
mount_options = ["nolocks"]
```

```bash
lsshfs --mount-option nobrowse @web01:/srv/data ./mnt/web01
```
