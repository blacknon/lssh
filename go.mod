module github.com/blacknon/lssh

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/blacknon/go-scplib v0.1.0
	github.com/blacknon/go-sshlib v0.1.1
	github.com/c-bata/go-prompt v0.2.3
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/kevinburke/ssh_config v0.0.0-20190724205821-6cfae18c12b8
	github.com/kr/pretty v0.1.0 // indirect
	github.com/mattn/go-colorable v0.1.2 // indirect
	github.com/mattn/go-runewidth v0.0.4
	github.com/nsf/termbox-go v0.0.0-20190325093121-288510b9734e
	github.com/sevlyar/go-daemon v0.1.5
	github.com/stretchr/testify v1.3.0
	github.com/urfave/cli v1.20.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4
	golang.org/x/net v0.0.0-20190420063019-afa5a82059c6
	mvdan.cc/sh v2.6.3+incompatible
)

replace github.com/blacknon/go-sshlib v0.1.1 => ../go-sshlib

go 1.13
