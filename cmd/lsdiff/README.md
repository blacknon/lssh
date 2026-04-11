lsdiff
======

`lsdiff` fetches remote files over SSH/SFTP and compares them side by side in a synchronized `tview` UI.

## Usage

```console
$ lsdiff --help
NAME:
    lsdiff - Compare remote files from multiple SSH hosts in a synchronized TUI.
USAGE:
    lsdiff [options] remote_path | @host:/path...

OPTIONS:
    --file filepath, -F filepath        config filepath. (default: "/Users/blacknon/.lssh.conf")
    --generate-lssh-conf ~/.ssh/config  print generated lssh config from OpenSSH config to stdout (~/.ssh/config by default).
    --help, -h                          print this help
    --enable-control-master             temporarily enable ControlMaster for this command execution
    --disable-control-master            temporarily disable ControlMaster for this command execution
    --version, -v                       print the version

VERSION:
    lssh-suite 0.8.1 (alpha/unknown)

USAGE:
    # select multiple hosts and compare the same remote path
    lsdiff /etc/hosts

    # compare different paths on specific hosts
    lsdiff @host1:/etc/hosts @host2:/etc/hosts @host3:/tmp/hosts

```

Examples:

```bash
# select multiple hosts, compare the same remote path
lsdiff /etc/hosts

# compare explicit host/path combinations
lsdiff @host1:/etc/hosts @host2:/etc/hosts @host3:/tmp/hosts
```

## Keys

- `Up` / `Down`: synchronized vertical scroll across all panes
- `Left` / `Right`: synchronized horizontal scroll across all panes
- `/`: open search prompt
- `Esc`: clear search
- `Ctrl+Q`: quit

## Notes

- `lsdiff` requires at least two targets.
- In common-path mode, host selection uses the existing `lssh` host selector and requires selecting at least two hosts.
- The compare layout aligns lines using a vimdiff-like base-file alignment so three or more files can be viewed together.
