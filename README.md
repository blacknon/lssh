lssh
====

List selection formula ssh wrapper command

## Description

lssh is List selection formula ssh wrapper command

## Demo

<p align="center">
<img src="./example/lssh.gif" />
</p>

## Requirement

need the following command.

- ssh
- script (log enable only)
- awk (log enable only)

## Install

    go get github.com/blacknon/lssh
    go install github.com/blacknon/lssh
    cp $GOPATH/src/github.com/blacknon/lssh/example/config.tml ~/.lssh.conf
    chmod 600 ~/.lssh.conf

## Usage

Please edit "~/.lssh.conf".
config ex)

	[log]
	enable = true
	dirpath = "/path/to/logdir"

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


option

    usage: lssh [--filepath FILEPATH] [--exec EXEC]

	options:
	  --filepath FILEPATH, -f FILEPATH
	                         config file path [default: /home/blacknon/.lssh.conf]
	  --exec EXEC, -e EXEC   exec_command
	  --help, -h             display this help and exit


### Use list select type ssh gateway server


#### '/etc/passwd' use

To use as a ssh gateway server as list select type, specify it as an execution command with "/etc/passwd" or "authorized_keys"

ex) /etc/passwd

    lssh:x:1000:1000::/home/lssh:/bin/lssh

Arrange "~/.lssh.conf" and connect with ssh.

<p align="center">
<img src="./example/lssh_withpasswd.gif" />
</p>


#### '/etc/passwd' with 'tmux' use




## Licence

A short snippet describing the license [MIT](https://github.com/blacknon/lssh/blob/master/LICENSE.md).

## Author

[blacknon](https://github.com/blacknon)
