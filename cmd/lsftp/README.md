lsftp
===

<p align="center">
<img src="./img/lsftp.gif" width="720" />
</p>

## About

`lsftp` is an interactive SFTP shell that can connect to one or more hosts at the same time.
After selecting hosts, you can browse files, transfer data, and manage directories from a single prompt.

## Usage

```shell
$ lsftp --help
NAME:
    lsftp - TUI list select and parallel sftp client command.
USAGE:
    lsftp [options]

OPTIONS:
    --file value, -F value  config file path (default: "/Users/blacknon/.lssh.conf")
    --help, -h              print this help
    --version, -v           print the version

COPYRIGHT:
    blacknon(blacknon@orebibou.com)

VERSION:
    lssh-suite 0.8.0 (stable/transfer)

USAGE:
    # start lsftp shell
    lsftp

```

## OverView

### interactive shell

Start `lsftp`, select one or more hosts from the TUI list, and the `lsftp>>` prompt will open.
When multiple hosts are selected, many operations run in parallel against the selected hosts.

```bash
# start lsftp shell
lsftp

# specify the config file
lsftp -F ~/.lssh.toml
```

### file operations

`lsftp` supports both remote and local file operations from the same shell.

- Download files with `get`
- Upload files with `put`
- Sync files with `sync`
- Copy files between remote hosts with `copy`
- Browse files with `ls`, `tree`, and `df`
- Manage files with `mkdir`, `rm`, `rename`, `chmod`, `chown`, and related commands

Command examples.

`get` and `put` can transfer files in parallel with worker-based processing. Use `-P` to increase the number of workers when downloading or uploading many files.

```bash
# download from remote to local
get /var/log/app.log ./

# download with parallel workers
get -P 4 /data/*.gz ./backup/

# upload from local to remote
put ./dist/app /opt/app/

# upload with parallel workers
put -P 4 ./dist/*.tar.gz /opt/archive/

# sync local -> remote
sync --delete local:./site remote:/var/www/site

# sync remote -> local
sync remote:/var/lib/app local:./backup

# copy between remote hosts
copy @web01:/var/log/app.log @web02:/tmp/
```

### built-in commands

Main commands available in the current implementation.

```text
cat       print remote file contents
cd        change remote directory
chgrp     change remote file group
chmod     change remote file permissions
chown     change remote file owner
copy      copy files between remote hosts
df        show disk usage
get       download from remote to local
lcat      print local file contents
lcd       change local directory
lls       list local directory
lmkdir    create local directory
ln        create link
lpwd      print local working directory
ls        list remote directory
lumask    set local umask
mkdir     create remote directory
put       upload from local to remote
sync      one-way sync between local and remote paths
pwd       print remote working directory
rename    rename remote file
rm        remove remote file
rmdir     remove remote directory
symlink   create symbolic link
tree      show remote tree
ltree     show local tree
help, ?   show help
bye, exit, quit
```

### notes

Remote host notation for `copy` uses the `@host:/path` format.
For `sync`, use the `lssync` style `(local|remote):path` prefixes. You can target a specific remote host inside `lsftp` with `remote:@host:/path`.
The default config search order is `~/.lssh.toml`, `~/.lssh.yaml`, `~/.lssh.yml`, then `~/.lssh.conf`.
