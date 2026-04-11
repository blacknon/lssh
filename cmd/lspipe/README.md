# `lspipe`

`lspipe` keeps a selected host set in the background and lets you reuse it from local shell pipelines.

Named pipe bridges created with `--mkfifo` are currently supported on Unix-like systems only.

## Examples

```bash
# create the default session from the host selector
lspipe

# create a named session from the CLI
lspipe --name prod -H web01 -H web02 -H web03

# run a command on every host in the session
lspipe hostname

# broadcast stdin to every host
echo test | lspipe 'cat'

# single-host raw mode for process substitution
vimdiff \
  <(lspipe -h web01 --raw cat /etc/hosts) \
  <(lspipe -h web02 --raw cat /etc/hosts)

# inspect or close a session
lspipe --list
lspipe --info --name prod
lspipe --close --name prod

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

## Notes

- `lspipe` sessions are single local handles to a chosen host set.
- `stdin` is broadcast to every selected host in the current MVP.
- `--raw` is only allowed when the resolved target set contains exactly one host.
- `--mkfifo` creates `all.*` pipes plus one `host.*` set per host: `.cmd`, `.stdin`, `.out`.
- Write stdin into `.stdin`, then write the remote command into `.cmd`; read the result from `.out`.
- `--mkfifo` is currently Unix-only. Windows is not supported yet.
