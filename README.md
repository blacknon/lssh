[![CircleCI](https://circleci.com/gh/blacknon/lssh.svg?style=svg)](https://circleci.com/gh/blacknon/lssh)

lssh
====

TUI list select ssh/scp client.

## Description

command to read a prepared list in advance and connect ssh/scp the selected host. List file is set in yaml format. When selecting a host, you can filter by keywords. Can execute commands concurrently to multiple hosts. Supported multiple ssh proxy, and supported http/socks5 proxy.

## Features

* List selection type ssh client.
* Pure Go.
* Commands can be executed by ssh connection in parallel.
* Supported multiple proxy.
* Can use bashrc of local machine at ssh connection destination.

## Demo

<p align="center">
<img src="./example/lssh.gif" />
</p>

## Requirement

lscp is need the following command in remote server.

- scp

## Install

compile gofile(tested go1.12.4).

    go get -u github.com/blacknon/lssh/cmd/lssh
    go get -u github.com/blacknon/lssh/cmd/lscp

or

    make && sudo make install

brew install(Mac OS X)

	brew tap blacknon/lssh
	brew install lssh

	# generate .lssh.conf(not use ~/.ssh/config)
	curl -s https://raw.githubusercontent.com/blacknon/lssh/master/example/config.tml | cp -n <(cat) ~/.lssh.conf # copy sample config file

	# generate .lssh.conf(use ~/.ssh/config.not support proxy)
	lssh --generate > ~/.lssh.conf

## Config

Please edit "~/.lssh.conf".\
For details see [wiki](https://github.com/blacknon/lssh/wiki/Config).

example:

	# terminal log settings
	[log]
	enable = true
	timestamp = true
	dirpath = "/path/to/logdir"

	# server common settings
	[common]
	port = "22"
	user = "test"

	# include config file settings and path (only common,server config).
	[include.Name]
	path = "/path/to/include/file"

	[server.PasswordAuth_ServerName]
	addr = "192.168.100.101"
	pass = "Password"
	note = "Password Auth Server"

	[server.KeyAuth_ServerName]
	addr = "192.168.100.101"
	user = "test-user" # user change
	key  = "/path/to/private_key"
	note = "Key Auth Server"

	[server.CertAuth_ServerName]
	addr = "192.168.100.101"
	user = "test-user" # user change
	cert  = "/path/to/cert"
	certkey  = "/path/to/private_key"
	note = "Cert Auth Server"

	[server.AuthPkcs11_ServerName]
	addr = "192.168.100.101"
	pkcs11provider = "/usr/local/lib/opensc-pkcs11.so"
	pkcs11pin = "123456" # option: pkcs11 pin code
	note = "PKCS11 Auth Server"

	[server.LocalCommand_ServerName]
	addr = "192.168.100.103"
	key  = "/path/to/private_key"
	note = "Before/After run local command"
	pre_cmd = "(option) exec command before ssh connect."
	post_cmd = "(option) exec command after ssh disconnected."

	[server.UseLocalBashrc_ServerName]
	addr = "192.168.100.104"
	key  = "/path/to/private_key"
	note = "Use local bashrc files."
	pre_cmd = "(option) exec command before ssh connect."
	local_rc = 'yes'
	local_rc_file = [
         "~/dotfiles/.bashrc"
        ,"~/dotfiles/bash_prompt"
        ,"~/dotfiles/sh_alias"
        ,"~/dotfiles/sh_export"
        ,"~/dotfiles/sh_function"
	]

	[server.sshProxyServer]
	addr = "192.168.100.200"
	key  = "/path/to/private_key"
	note = "proxy server"

	[server.overProxyServer]
	addr = "192.168.10.10"
	key  = "/path/to/private_key"
	note = "connect use ssh proxy"
	proxy = "sshProxyServer"

	[server.overProxyServer2]
	addr = "192.168.10.100"
	key  = "/path/to/private_key"
	note = "connect use ssh proxy(multiple)"
	proxy = "overProxyServer"

	[server.overHttpProxy]
	addr = "over-http-proxy.com"
	key  = "/path/to/private_key"
	note = "connect use http proxy"
	proxy = "HttpProxy"
	proxy_type = "http"

	[server.overSocks5Proxy]
	addr = "192.168.10.101"
	key  = "/path/to/private_key"
	note = "connect use socks5 proxy"
	proxy = "Socks5Proxy"
	proxy_type = "socks5"

	[proxy.HttpProxy]
	addr = "example.com"
	port = "8080"

	[proxy.Socks5Proxy]
	addr = "example.com"
	port = "54321"


## Usage

run command.

    lssh


option(lssh)

	NAME:
	    lssh - TUI list select and parallel ssh client command.
	USAGE:
	    lssh [options] [commands...]
	
	OPTIONS:
	    --host value, -H value      connect servernames
	    --file value, -f value      config file path (default: "/Users/blacknon/.lssh.conf")
	    --portforward-local value   port forwarding local port(ex. 127.0.0.1:8080)
	    --portforward-remote value  port forwarding remote port(ex. 127.0.0.1:80)
	    --list, -l                  print server list from config
	    --term, -t                  run specified command at terminal
	    --parallel, -p              run command parallel node(tail -F etc...)
	    --generate                  (beta) generate .lssh.conf from .ssh/config.(not support ProxyCommand)
	    --help, -h                  print this help
	    --version, -v               print the version
	
	COPYRIGHT:
	    blacknon(blacknon@orebibou.com)
	
	VERSION:
	    0.5.4
	
	USAGE:
	    # connect ssh
	    lssh
	
	    # parallel run command in select server over ssh
	    lssh -p command...


	option(lscp)
	
	NAME:
	    lscp - TUI list select and parallel scp client command.
	USAGE:
	    lscp [options] (local|remote):from_path... (local|remote):to_path
	    
	OPTIONS:
	    --host value, -H value  connect servernames
	    --list, -l              print server list from config
	    --file value, -f value  config file path (default: "/home/blacknon/.lssh.conf")
	    --permission, -p        copy file permission
	    --help, -h              print this help
	    --version, -v           print the version
	    
	COPYRIGHT:
	    blacknon(blacknon@orebibou.com)
	    
	VERSION:
	    0.5.4
	    
	USAGE:
	    # local to remote scp
	    lscp /path/to/local... /path/to/remote
	
	    # remote to local scp
	    lscp remote:/path/to/remote... /path/to/local
	                              

If you specify a command as an argument, you can select multiple hosts. Select host 'Tab', select all displayed hosts 'Ctrl + A'.

### [lssh] copy files using stdin/stdout, and to/from remote server

You can scp like copy files using stdin/stdout.It also supports multiple nodes(parallel is not yet supported now).

	# from local to remote server
	cat LOCAL_PATH | lssh 'cat > REMOTE_PATH'

	# from remote server to local
	lssh cat REMOTE_PATH | cat > LOCAL_PATH

<p align="center">
<img src="./example/lssh_stdcp.gif" />
</p>

### [lssh] multiple node select, exec tail -f

	# -p option parallel exec command
	lssh -p cmd


<p align="center">
<img src="./example/lssh_parallel.gif" />
</p>

### [lssh] ssh connect, and change Terminal theme.

sample lssh.conf

<p align="center">
<img src="./example/lssh_iterm2.gif" />
</p>

    [server.iTerm2_sample]
	addr = "192.168.100.103"
	key  = "/path/to/private_key"
	note = "Before/After run local command"
	pre_cmd = 'printf "\033]50;SetProfile=dq\a"' # ssh theme
    post_cmd = 'printf "\033]50;SetProfile=Default\a"' # local theme
	post_cmd = "(option) exec command after ssh disconnected."

    [server.GnomeTerminal_sample]
	addr = "192.168.100.103"
	key  = "/path/to/private_key"
	note = "Before/After run local command"
	pre_cmd = 'printf "\033]50;SetProfile=dq\a"' # ssh theme
    post_cmd = 'printf "\033]50;SetProfile=Default\a"' # local theme
	post_cmd = "(option) exec command after ssh disconnected."


### [lssh] use local bashrc file.

sample lssh.conf(bash only).

    [server.localrc]
	addr = "192.168.100.104"
	key  = "/path/to/private_key"
	note = "Use local bashrc files."
	local_rc = 'yes'
	local_rc_file = [
         "~/dotfiles/.bashrc"
        ,"~/dotfiles/bash_prompt"
        ,"~/dotfiles/sh_alias"
        ,"~/dotfiles/sh_export"
        ,"~/dotfiles/sh_function"
	]


### [lscp] scp remote to local (get)

exec lscp get file/dir (remote to local scp).

	lscp remote:/path/to/remote local:/path/to/local
	
	# short version
	lscp r:/path/to/remote l:/path/to/local
	lscp r:/path/to/remote /path/to/local


### [lscp] scp local to remote (put)

exec lscp put file/dir (local to remote scp). If multiple server selected, mkdir servername dir.

	lscp local:/path/to/remote remote:/path/to/local
	
	# short version
	lscp l:/path/to/local r:/path/to/remote
	lscp /path/to/local r:/path/to/remote


### [lscp] scp remote to remote

exec lscp get/put file/dir (remote to remote scp).

	lscp remote:/path/to/remote(get) remote:/path/to/remote(put)
	
	# short version
	lscp r:/path/to/remote(get) r:/path/to/local(put)


## Licence

A short snippet describing the license [MIT](https://github.com/blacknon/lssh/blob/master/LICENSE.md).

## Author

[blacknon](https://github.com/blacknon)
