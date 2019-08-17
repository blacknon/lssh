// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/blacknon/lssh/conf"
	"github.com/sevlyar/go-daemon"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

// TODO(blacknon): 自動再接続機能の追加(v0.6.1)
//     autosshのように、接続が切れた際に自動的に再接続を試みる動作をさせたい
//     パラメータでの有効・無効指定が必要になる。

// TODO(blacknon):

// TODO(blacknon): リバースでのsshfsの追加(v0.6.1以降？)
//     lsshfs実装後になるか？ssh接続時に、指定したフォルダにローカルの内容をマウントさせて読み取らせる。
//     うまくやれれば、ローカルのスクリプトなどをそのままマウントさせて実行させたりできるかもしれない。
//     Socketかなにかでトンネルさせて、あとは指定したディレクトリ配下をそのままFUSEでファイルシステムとして利用できるように書けばいける…？
//
//     【参考】
//         - https://github.com/rom1v/rsshfs
//         - https://github.com/hanwen/go-fuse
//         - https://gitlab.com/dns2utf8/revfs/

type Run struct {
	ServerList []string
	Conf       conf.Config

	// Mode value in
	//     - shell
	//     - cmd
	//     - pshell
	Mode string

	// tty use (-t option)
	IsTerm bool

	// parallel connect (-p option)
	IsParallel bool

	// not run (-N option)
	IsNone bool

	// x11 forwarding (-X option)
	X11 bool

	// use or not-use local bashrc.
	// IsNotBashrc takes precedence.
	IsBashrc    bool
	IsNotBashrc bool

	// local/remote Port Forwarding
	PortForwardMode   string // L or R
	PortForwardLocal  string
	PortForwardRemote string

	// Dynamic Port Forwarding
	// set localhost port num (ex. 11080).
	DynamicPortForward string

	// Exec command
	ExecCmd []string

	// Agent is ssh-agent.
	// In agent.Agent or agent.ExtendedAgent.
	agent interface{}

	// StdinData from pipe flag
	isStdinPipe bool

	// AuthMethodMap is
	// map of AuthMethod summarized in Run overall
	authMethodMap map[AuthKey][]ssh.AuthMethod

	// ServerAuthMethodMap is
	// Map of AuthMethod used by target server
	serverAuthMethodMap map[string][]ssh.AuthMethod
}

// Auth map key
type AuthKey struct {
	// auth type:
	//   - password
	//   - agent
	//   - key
	//   - cert
	//   - pkcs11
	Type string

	// auth type value:
	//   - key(path)
	//     ex.) ~/.ssh/id_rsa
	//   - cert(path)
	//     ex.) ~/.ssh/id_rsa.crt
	//   - pkcs11(libpath)
	//     ex.) /usr/local/lib/opensc-pkcs11.so
	Value string
}

// use scp, sftp
type CopyConInfo struct {
	IsRemote bool
	Path     []string
	Server   []string
}

const (
	AUTHKEY_PASSWORD = "password"
	AUTHKEY_AGENT    = "agent"
	AUTHKEY_KEY      = "key"
	AUTHKEY_CERT     = "cert"
	AUTHKEY_PKCS11   = "pkcs11"
)

// Start ssh connect
func (r *Run) Start() {
	var err error

	// Get stdin data(pipe)
	// TODO(blacknon): os.StdinをReadAllで全部読み込んでから処理する方式だと、ストリームで処理出来ない
	//                 (全部読み込み終わるまで待ってしまう)ので、Reader/Writerによるストリーム処理に切り替える(v0.6.0)
	//                 => flagとして検知させて、あとはpushPipeWriterにos.Stdinを渡すことで対処する
	if runtime.GOOS != "windows" {
		stdin := 0
		if !terminal.IsTerminal(stdin) {
			r.isStdinPipe = true
		}
	}

	// create AuthMap
	r.createAuthMethodMap()

	// connect
	switch {
	case len(r.ExecCmd) > 0 && r.Mode == "cmd":
		// connect and run command
		err = r.cmd()

	case r.Mode == "shell":
		// connect remote shell
		err = r.shell()

	case r.Mode == "pshell":
		// start lsshshell
		err = r.pshell()

	default:
		return
	}

	if err != nil {
		fmt.Println(err)
	}
}

// printSelectServer is printout select server.
// use ssh login header.
func (r *Run) printSelectServer() {
	serverListStr := strings.Join(r.ServerList, ",")
	fmt.Fprintf(os.Stderr, "Select Server :%s\n", serverListStr)
}

// printRunCommand is printout run command.
// use ssh command run header.
func (r *Run) printRunCommand() {
	runCmdStr := strings.Join(r.ExecCmd, " ")
	fmt.Fprintf(os.Stderr, "Run Command   :%s\n", runCmdStr)
}

// printPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printPortForward(m, forwardLocal, forwardRemote string) {
	if forwardLocal != "" && forwardRemote != "" {
		var mode, arrow string
		switch m {
		case "L", "":
			mode = "LOCAL "
			arrow = " =>"
		case "R":
			mode = "REMOTE"
			arrow = "<= "
		}

		fmt.Fprintf(os.Stderr, "Port Forward  :%s\n", mode)
		fmt.Fprintf(os.Stderr, "               local[%s] %s remote[%s]\n", forwardLocal, arrow, forwardRemote)
	}
}

// printPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printDynamicPortForward(port string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "DynamicForward:%s\n", port)
		fmt.Fprintf(os.Stderr, "               %s\n", "connect Socks5.")
	}
}

// printProxy is printout proxy route.
// use ssh command run header. only use shell().
func (r *Run) printProxy(server string) {
	array := []string{}

	proxyRoute, err := getProxyRoute(server, r.Conf)
	if err != nil || len(proxyRoute) == 0 {
		return
	}

	// set localhost
	localhost := "localhost"

	// set target host
	targethost := server

	// add localhost
	array = append(array, localhost)

	for _, pxy := range proxyRoute {
		// seprator
		var sep string
		if pxy.Type == "command" {
			sep = ":"
		} else {
			sep = "://"
		}

		// setup string
		str := "[" + pxy.Type + sep + pxy.Name
		if pxy.Port != "" {
			str = str + ":" + pxy.Port
		}
		str = str + "]"

		array = append(array, str)
	}

	// add target
	array = append(array, targethost)

	// print header
	header := strings.Join(array, " => ")
	fmt.Fprintf(os.Stderr, "Proxy         :%s\n", header)
}

// runCmdLocal exec command local machine.
// Mainly used in r.shell().
func execLocalCommand(cmd string) {
	out, _ := exec.Command("sh", "-c", cmd).CombinedOutput()
	fmt.Printf(string(out))
}

// startBackgroundMode run deamon mode
// not working... and not use... how this do ...?
func startBackgroundMode() {
	cntxt := &daemon.Context{}
	d, err := cntxt.Reborn()
	if err != nil {
		log.Fatalln(err)
	}

	if d != nil {
		return
	}

	defer func() {
		if err := cntxt.Release(); err != nil {
			log.Printf("error encountered while killing daemon: %v", err)
		}
	}()

	return
}
