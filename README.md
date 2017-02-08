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
	key  = "/tmp/key.pem"
	note = "Key Auth Server"


After exec command.

    lssh


### Use springboard server


To use as a steppingstone server, specify it as an execution command with "/etc/passwd" or "authorized_keys"

/etc/passwd ex)

    lssh:x:1000:1000::/home/lssh:/bin/lssh

Arrange "~/.lssh.conf" and connect with ssh.

<p align="center">
<img src="./example/lssh_withpasswd.gif" />
</p>


## Licence

A short snippet describing the license [MIT](https://github.com/tcnksm/tool/blob/master/LICENCE).

## Author

[blacknon](https://github.com/blacknon)
