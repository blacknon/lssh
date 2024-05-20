module github.com/blacknon/lssh

go 1.22.2

// require
require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/BurntSushi/toml v0.3.1
	github.com/ScaleFT/sshkeys v0.0.0-20200327173127-6142f742bca5 // indirect
	github.com/ThalesIgnite/crypto11 v1.2.5 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/armon/go-socks5 v0.0.0-20160902184237-e75332964ef5 // indirect
	github.com/blacknon/go-sshlib v0.1.11
	github.com/blacknon/go-x11auth v0.1.0 // indirect
	github.com/blacknon/textcol v0.0.1
	github.com/c-bata/go-prompt v0.2.6
	github.com/davecgh/go-spew v1.1.0 // indirect
	github.com/dchest/bcrypt_pbkdf v0.0.0-20150205184540-83f37f9c154a // indirect
	github.com/disiqueira/gotree v1.0.0
	github.com/dustin/go-humanize v1.0.0
	github.com/elazarl/goproxy v0.0.0-20231117061959-7cc037d33fb5 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/kevinburke/ssh_config v0.0.0-20190724205821-6cfae18c12b8
	github.com/kr/fs v0.1.0 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/lunixbochs/vtclean v1.0.0 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/mattn/go-runewidth v0.0.13
	github.com/mattn/go-tty v0.0.3 // indirect
	github.com/miekg/pkcs11 v1.1.1 // indirect
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6 // indirect
	github.com/nsf/termbox-go v1.1.1
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/sftp v1.13.4
	github.com/pkg/term v1.2.0-beta.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/sevlyar/go-daemon v0.1.5
	github.com/stretchr/testify v1.7.1
	github.com/thales-e-security/pool v0.0.2 // indirect
	github.com/urfave/cli v1.21.0
	github.com/vbauerster/mpb v3.4.0+incompatible
	golang.org/x/crypto v0.21.0
	golang.org/x/net v0.23.0
	golang.org/x/sys v0.19.0
	golang.org/x/term v0.19.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	mvdan.cc/sh v2.6.3+incompatible
)

// replace
replace (
	github.com/ThalesIgnite/crypto11 v1.2.5 => github.com/blacknon/crypto11 v1.2.6
	github.com/c-bata/go-prompt v0.2.6 => github.com/blacknon/go-prompt v0.2.7
)
