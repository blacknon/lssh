lsmon
===

## About

`lsmon` is a TUI monitor for watching multiple remote hosts side by side.
It connects over SSH and shows system information such as CPU, memory, disk, network, and process status in one screen.

## Usage

```shell
$ lsmon --help
NAME:
    lsmon - TUI list select and parallel ssh monitoring command.
USAGE:
    lsmon [options] [commands...]

OPTIONS:
    --host servername, -H servername    connect servername.
    --file filepath, -F filepath        config filepath. (default: "/Users/blacknon/.lssh.conf")
    --logfile value, -L value           Set log file path.
    --list, -l                          print server list from config.
    --debug                             debug pprof. use port 6060.
    --help, -h                          print this help
    --version, -v                       print the version

COPYRIGHT:
    blacknon(blacknon@orebibou.com)

VERSION:
    lssh-suite 0.7.0 (stable/core)

USAGE:
    # connect parallel ssh monitoring command
    lsmon
```

## OverView

### monitor targets

`lsmon` can monitor multiple hosts selected from the TUI list, or you can specify them directly with `-H`.
It is designed for comparing host state across a server list.

```bash
# start monitoring after selecting hosts from the TUI
lsmon

# specify hosts directly
lsmon -H web01 -H web02
```

### metrics

The monitor displays the following kinds of information

- uptime
- load average
- CPU usage and core count
- memory and swap usage
- disk usage and disk I/O
- network throughput and packet counts
- process information

### logging and debug

You can write logs to a file with `-L`.
You can also enable `pprof` on `localhost:6060` with `--debug`.

```bash
# write monitor logs to a file
lsmon -L ./lsmon.log

# enable pprof for debugging
lsmon --debug
```

### notes

The default config file path is `~/.lssh.conf`.
If no log file is specified, logs are written to `/dev/null`.

Most data collection assumes Linux-style `/proc` information on the remote side, so in practice `lsmon` is aimed at Linux hosts.
The SSH connect timeout is set to 5 seconds in the current implementation.

