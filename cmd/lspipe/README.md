# `lspipe`

## About

`lspipe` keeps a selected host set in the background and lets you reuse it from local shell pipelines.
It is designed for cases where you want to choose hosts once, keep that session alive, and then run multiple commands through the same selection later.

Session-based execution works on Linux, macOS, and Windows.
Named pipe bridges created with `--mkfifo` are currently supported on Unix-like systems only.

## Usage

```shell
$ lspipe --help
NAME:
    lspipe - Persistent SSH pipe sessions for reusing selected hosts from local shell pipelines.
USAGE:
    lspipe [options] [command...]
    
OPTIONS:
    --name name                              session name. (default: "default")
    --fifo-name name                         named pipe set name. (default: "default")
    --create-host servername, -H servername  add servername when creating or replacing a session.
    --host servername, -h servername         limit command execution to servername inside the session.
    --file filepath, -F filepath             config filepath. (default: "/Users/blacknon/.lssh.conf")
    --generate-lssh-conf ~/.ssh/config       print generated lssh config from OpenSSH config to stdout (~/.ssh/config by default).
    --replace                                replace the named session if it already exists.
    --list                                   list known lspipe sessions.
    --mkfifo                                 create a named pipe bridge for the named session.
    --list-fifos                             list named pipe bridges.
    --rmfifo                                 remove the named pipe bridge for the named session.
    --info                                   show information for the named session.
    --close                                  close the named session.
    --raw                                    write pure stdout for exactly one resolved host.
    --help                                   print this help
    --enable-control-master                  temporarily enable ControlMaster for this command execution
    --disable-control-master                 temporarily disable ControlMaster for this command execution
    --version, -v                            print the version
    
VERSION:
    lssh-suite 0.9.0 (alpha/sysadmin)
    
USAGE:
    # create default session from TUI
    lspipe

    # create named session from cli
    lspipe --name prod -H web01 -H web02

    # execute command through existing session
    lspipe hostname
    echo test | lspipe 'cat'

    # single host raw output
    lspipe -h web01 --raw cat /etc/hosts
```

## Overview

### create and reuse a background session

When `lspipe` is run without a command, it creates or reuses a named background session.
That session stores the selected host set and stays available for later commands, so repeated work can be scripted from the local shell without reopening the host selector each time.

```bash
# create the default session from the host selector
lspipe

# create a named session from the CLI
lspipe --name prod -H web01 -H web02 -H web03
```

### execute commands through the session

Once a session exists, you can send commands through it and optionally broadcast `stdin`.
Use `--host` when you want to limit execution to a subset of the session, and `--raw` when you need plain stdout from exactly one resolved host.

```bash
# run a command on every host in the session
lspipe hostname

# broadcast stdin to every host
echo test | lspipe 'cat'

# single-host raw mode for process substitution
vimdiff \
  <(lspipe -h web01 --raw cat /etc/hosts) \
  <(lspipe -h web02 --raw cat /etc/hosts)
```

### inspect and close sessions

```bash
# inspect or close a session
lspipe --list
lspipe --info --name prod
lspipe --close --name prod
```

### create fifo bridges

On Unix-like systems, `--mkfifo` creates named pipes that let other local processes feed commands and stdin into a session without invoking `lspipe` each time.
This is useful when integrating the session with shell scripts or long-running local automation.

```bash
# create a named pipe bridge
lspipe --mkfifo --fifo-name ops

# read from the aggregate output pipe
cat ~/.cache/lssh/lspipe/fifo/default/ops/all.out

# send a command to all hosts in the session
echo hostname > ~/.cache/lssh/lspipe/fifo/default/ops/all.cmd

# send stdin for the next command, then run it on a single host pipe
printf 'hello from fifo\n' > ~/.cache/lssh/lspipe/fifo/default/ops/web01.stdin
echo 'cat' > ~/.cache/lssh/lspipe/fifo/default/ops/web01.cmd
```

### notes

- `lspipe` sessions are single local handles to a chosen host set.
- `stdin` is broadcast to every selected host in the current MVP.
- `--raw` is only allowed when the resolved target set contains exactly one host.
- Windows supports normal `lspipe` session creation and command execution through the local TCP fallback.
- `--mkfifo` creates `all.*` pipes plus one `host.*` set per host: `.cmd`, `.stdin`, `.out`.
- Write stdin into `.stdin`, then write the remote command into `.cmd`; read the result from `.out`.
- `--mkfifo` is currently Unix-only. Windows does not support the FIFO bridge workflow in `0.9.0`.
- The default config search order is `~/.lssh.toml`, `~/.lssh.yaml`, `~/.lssh.yml`, then `~/.lssh.conf`.
