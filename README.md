[![CircleCI](https://circleci.com/gh/blacknon/lssh.svg?style=svg)](https://circleci.com/gh/blacknon/lssh)

lssh
====

TUI list select ssh/scp client.

## Description

command to read a prepared list in advance and connect ssh/scp the selected host. List file is set in yaml format.When selecting a host, you can filter by keywords. Can execute commands concurrently to multiple hosts.

## Demo

<p align="center">
<img src="./example/lssh.gif" />
</p>

## Requirement

need the following command.

- ssh
- scp (remote host)

## Install

    go get github.com/blacknon/lssh
    go install github.com/blacknon/lssh
    cp $GOPATH/src/github.com/blacknon/lssh/example/config.tml ~/.lssh.conf
    chmod 600 ~/.lssh.conf

## Usage

Please edit "~/.lssh.conf".The connection information at servers,can be divided into external files.

example:

	[log]
	enable = true
	dirpath = "/path/to/logdir"

	[include.Name]
	path = "/path/to/include/file"

	[server.PasswordAuth_ServerName]
	addr = "192.168.100.101"
	port = "22"
	user = "test"
	pass = "Password"
	note = "Password Auth Server"

	[server.KeyAuth_ServerName]
	addr = "192.168.100.102"
	port = "22"
	user = "test"
	key  = "/path/to/private_key"
	note = "Key Auth Server"


After exec command.

    lssh


option(lssh)

	lssh v0.4.3
	Usage: lssh [--host HOST] [--list] [--file FILE] [--terminal] [--parallel] [--command COMMAND]

	Options:
	  --host HOST, -H HOST   Connect servername
	  --list, -l             print server list
	  --file FILE, -f FILE   config file path [default: /Users/uesugi/.lssh.conf]
	  --terminal, -t         Run specified command at terminal
	  --parallel, -p         Exec command parallel node(tail -F etc...)
	  --command COMMAND, -c COMMAND
                         Remote Server exec command.
	  --help, -h             display this help and exit
	  --version              display version and exit

option(lscp)

	lscp v0.4.3
	Usage: lscp [--host HOST] [--file FILE] FROM TO

	Positional arguments:
	  FROM                   Copy from path
	  TO                     Copy to path

	Options:
	  --host HOST, -H HOST   Connect servername
	  --file FILE, -f FILE   config file path [default: /Users/uesugi/.lssh.conf]
	  --help, -h             display this help and exit
	  --version              display version and exit

If you specify a command as an argument, you can select multiple hosts. Select host 'Tab', select all displayed hosts 'Ctrl + A'.

### copy files using stdin/stdout to/from remote server

You can scp like copy files using stdin/stdout.It also supports multiple nodes(parallel is not yet supported now).

	# from local to remote server
	cat LOCAL_PATH | lssh -C 'cat > REMOTE_PATH'

	# from remote server to local
	lssh -C 'cat REMOTE_PATH' | cat > LOCAL_PATH

<p align="center">
<img src="./example/lssh_stdcp.gif" />
</p>


### multiple node select exec tail -f

<p align="center">
<img src="./example/lssh_parallel.gif" />
</p>

### Use list select type ssh gateway server

#### '/etc/passwd' use (or .ssh/authorized_keys)

To use as a ssh gateway server as list select type, specify it at an execution command in "/etc/passwd"( or "authorized_keys").

example /etc/passwd

    lssh:x:1000:1000::/home/lssh:/bin/lssh

Arrange "~/.lssh.conf" and connect with ssh.

<p align="center">
<img src="./example/lssh_withpasswd.gif" />
</p>

## Licence

A short snippet describing the license [MIT](https://github.com/blacknon/lssh/blob/master/LICENSE.md).

## Author

[blacknon](https://github.com/blacknon)
