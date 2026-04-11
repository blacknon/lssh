lsshfs
======

`lsshfs` mounts a remote directory from a single host in your `lssh` inventory.

- Linux clients use `FUSE`
- macOS clients use `NFS`
- Windows clients use `SMB`

The command runs in the background by default and automatically unmounts when the SSH connection is lost.

## Usage

```console
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
    lssh-suite 0.8.1 (alpha/unknown)

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

Examples:

```bash
lsshfs @web01:/var/www ./mnt/web01
lsshfs -H web01 /var/www ./mnt/web01
lsshfs --rw @web01:/srv/app ./mnt/app
lsshfs --unmount ./mnt/app
lsshfs --list-mounts
```

Windows uses a drive letter as the mount target:

```powershell
lsshfs @web01:/srv/data Z:
```

## Notes

- `lsshfs` supports only one host at a time.
- `@host:/path` is the preferred remote path format, but `host:/path` is still accepted for compatibility.
- On macOS, the local mount is created with `mount_nfs`.
- On Windows, the local mount is created with `net use` against `\\127.0.0.1\share`.
- On Windows, binding the local SMB listener to port `445` may require elevated privileges depending on the environment.
