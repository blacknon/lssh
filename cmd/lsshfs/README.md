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
