module github.com/blacknon/lssh

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/VividCortex/ewma v1.1.1 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/blacknon/go-sshlib v0.1.3
	github.com/blacknon/textcol v0.0.1
	github.com/c-bata/go-prompt v0.2.3
	github.com/dustin/go-humanize v1.0.0
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/kevinburke/ssh_config v0.0.0-20190724205821-6cfae18c12b8
	github.com/kr/pretty v0.1.0 // indirect
	github.com/mattn/go-colorable v0.1.2 // indirect
	github.com/mattn/go-runewidth v0.0.4
	github.com/mattn/go-tty v0.0.0-20190424173100-523744f04859 // indirect
	github.com/nsf/termbox-go v0.0.0-20190325093121-288510b9734e
	github.com/pkg/sftp v1.10.1
	github.com/pkg/term v0.0.0-20190109203006-aa71e9d9e942 // indirect
	github.com/sevlyar/go-daemon v0.1.5
	github.com/stretchr/testify v1.5.1
	github.com/urfave/cli v1.21.0
	github.com/vbauerster/mpb v3.4.0+incompatible
	golang.org/x/crypto v0.0.0-20201016220609-9e8e0b390897
	golang.org/x/net v0.0.0-20201026091529-146b70c837a4
	mvdan.cc/sh v2.6.3+incompatible
)

replace github.com/urfave/cli v1.22.0 => ../../urfave/cli

replace github.com/miekg/pkcs11 => github.com/blacknon/pkcs11 v1.0.4-0.20201018135904-6038e308f617

go 1.13
