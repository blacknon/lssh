module github.com/blacknon/lssh

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/blacknon/go-sshlib v0.1.4
	github.com/blacknon/textcol v0.0.1
	github.com/c-bata/go-prompt v0.2.5
	github.com/dustin/go-humanize v1.0.0
	github.com/kevinburke/ssh_config v0.0.0-20190724205821-6cfae18c12b8
	github.com/mattn/go-runewidth v0.0.13
	github.com/nsf/termbox-go v0.0.0-20190325093121-288510b9734e
	github.com/pkg/sftp v1.10.1
	github.com/sevlyar/go-daemon v0.1.5
	github.com/stretchr/testify v1.5.1
	github.com/urfave/cli v1.21.0
	github.com/vbauerster/mpb v3.4.0+incompatible
	golang.org/x/crypto v0.0.0-20201124201722-c8d3bf9c5392
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	mvdan.cc/sh v2.6.3+incompatible
)

require (
	github.com/ScaleFT/sshkeys v0.0.0-20200327173127-6142f742bca5 // indirect
	github.com/ThalesIgnite/crypto11 v1.2.5 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/armon/go-socks5 v0.0.0-20160902184237-e75332964ef5 // indirect
	github.com/davecgh/go-spew v1.1.0 // indirect
	github.com/dchest/bcrypt_pbkdf v0.0.0-20150205184540-83f37f9c154a // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/mattn/go-tty v0.0.3 // indirect
	github.com/miekg/pkcs11 v1.0.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/term v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/thales-e-security/pool v0.0.2 // indirect
	golang.org/x/sys v0.0.0-20210927094055-39ccf1dd6fa6 // indirect
	golang.org/x/term v0.0.0-20201117132131-f5c789dd3221 // indirect
	gopkg.in/yaml.v2 v2.2.2 // indirect
)

replace github.com/urfave/cli v1.22.0 => ../../urfave/cli

// replace github.com/blacknon/go-sshlib v1.22.0 => ../go-sshlib

replace github.com/miekg/pkcs11 => github.com/blacknon/pkcs11 v1.0.4-0.20201018135904-6038e308f617

go 1.17
