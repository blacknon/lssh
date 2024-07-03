// Copyright (c) 2022 Blacknon. All rights reserved.
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

	"github.com/blacknon/lssh/common"
	"github.com/blacknon/lssh/conf"
	"github.com/sevlyar/go-daemon"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

// TODO(blacknon): 自動再接続機能の追加(v1.0.0)
//     autosshのように、接続が切れた際に自動的に再接続を試みる動作をさせたい
//     パラメータでの有効・無効指定が必要になる。
//     → go-sshlib側で処理させる

// TODO(blacknon): リバースでのsshfsの追加(v1.0.0以降？)
//     lsshfs実装後になるか？ssh接続時に、指定したフォルダにローカルの内容をマウントさせて読み取らせる。
//     うまくやれれば、ローカルのスクリプトなどをそのままマウントさせて実行させたりできるかもしれない。
//     Socketかなにかでトンネルさせて、あとは指定したディレクトリ配下をそのままFUSEでファイルシステムとして利用できるように書けばいける…？
//
//     【参考】
//         - https://github.com/rom1v/rsshfs
//         - https://github.com/hanwen/go-fuse
//         - https://gitlab.com/dns2utf8/revfs/
//
//     → go-sshlib側で処理させる(sshfsもreverse sshfsも共に)

// Run
type Run struct {
	ServerList []string
	Conf       conf.Config

	// Mode value in
	//     - shell
	//     - cmd
	Mode string

	// tty use (-t option)
	IsTerm bool

	// parallel connect (-p option)
	IsParallel bool

	// not run (-N option)
	IsNone bool

	// x11 forwarding (-X option)
	X11 bool

	// Trusted X11 flag (-Y option)
	X11Trusted bool

	// use or not-use local bashrc.
	// IsNotBashrc takes precedence.
	IsBashrc    bool
	IsNotBashrc bool

	// local/remote Port Forwarding
	PortForward []*conf.PortForward

	// TODO(blacknon): Delete old keys
	// L or R
	PortForwardMode string

	//
	PortForwardLocal string

	//
	PortForwardRemote string

	// Dynamic Port Forwarding
	// set localhost port num (ex. 11080).
	DynamicPortForward string

	// HTTP Dynamic Port Forwarding
	// set localhost port num (ex. 11080).
	HTTPDynamicPortForward string

	// Reverse Dynamic Port Forwarding
	// set remotehost port num (ex. 11080).
	ReverseDynamicPortForward string

	// HTTP Reverse Dynamic Port Forwarding
	// set remotehost port num (ex. 11080).
	HTTPReverseDynamicPortForward string

	// Exec command
	ExecCmd []string

	// enable/disable print header in command mode
	EnableHeader  bool
	DisableHeader bool

	// Agent is ssh-agent.
	// In agent.Agent or agent.ExtendedAgent.
	agent interface{}

	// StdinData from pipe flag
	IsStdinPipe bool

	// AuthMethodMap is
	// map of AuthMethod summarized in Run overall
	authMethodMap map[AuthKey][]ssh.AuthMethod

	// ServerAuthMethodMap is
	// Map of AuthMethod used by target server
	serverAuthMethodMap map[string][]ssh.AuthMethod

	// donedPKCS11 is　the value of panic measures (v0.6.2-).
	// If error occurs and pkcs11 processing occurs more than once, the library will keep the token and Panic will occur.
	// this value is so for countermeasures.
	donedPKCS11 bool
}

// AuthKey Auth map key struct.
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

// use scp,sftp
type PathSet struct {
	Base      string
	PathSlice []string
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
	//                 (全部読み込み終わるまで待ってしまう)ので、Reader/Writerによるストリーム処理に切り替える(v0.7.0)
	//                 => flagとして検知させて、あとはpushPipeWriterにos.Stdinを渡すことで対処する
	if runtime.GOOS != "windows" {
		stdin := 0
		if !terminal.IsTerminal(stdin) {
			r.IsStdinPipe = true
		}
	}

	// create AuthMap
	r.CreateAuthMethodMap()

	// connect
	switch {
	case len(r.ExecCmd) > 0 && r.Mode == "cmd":
		// connect and run command
		err = r.cmd()

	case r.Mode == "shell":
		// connect remote shell
		err = r.shell()

	default:
		return
	}

	if err != nil {
		fmt.Println(err)
	}
}

// PrintSelectServer is printout select server.
// use ssh login header.
func (r *Run) PrintSelectServer() {
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

// printDynamicPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printDynamicPortForward(port string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "DynamicForward:%s\n", port)
		fmt.Fprintf(os.Stderr, "               %s\n", "connect Socks5.")
	}
}

// printReverseDynamicPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printReverseDynamicPortForward(port string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "ReverseDynamicForward:%s\n", port)
		fmt.Fprintf(os.Stderr, "                      %s\n", "connect Socks5.")
	}
}

// printPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printHTTPDynamicPortForward(port string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "HTTPDynamicForward:%s\n", port)
		fmt.Fprintf(os.Stderr, "                   %s\n", "connect http.")
	}
}

// printHTTPReverseDynamicPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printHTTPReverseDynamicPortForward(port string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "HTTPReverseDynamicForward:%s\n", port)
		fmt.Fprintf(os.Stderr, "                        %s\n", "connect http.")
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

// setPortForwards is Add local/remote port forward to Run.PortForward
func (r *Run) setPortForwards(server string, config conf.ServerConfig) (c conf.ServerConfig) {
	// set config
	c = config

	// append single port forward settings (Backward compatibility).
	if c.PortForwardLocal != "" && c.PortForwardRemote != "" {
		fw := new(conf.PortForward)
		fw.Mode = c.PortForwardMode
		fw.Local = c.PortForwardLocal
		fw.Remote = c.PortForwardRemote

		c.Forwards = append(c.Forwards, fw)
	}

	// append port forwards from c, to r.PortForward
	for _, f := range c.PortForwards {
		var err error

		// create forward
		fw := new(conf.PortForward)

		// split config forward settings
		farray := strings.SplitN(f, ":", 2)

		// check array count
		if len(farray) == 1 {
			fmt.Fprintf(os.Stderr, "port forward format is incorrect: %s: \"%s\"", server, f)
			continue
		}

		//
		mode := strings.ToLower(farray[0])
		switch mode {
		// local/remote port forward
		case "local", "l":
			fw.Mode = "L"
			fw.Local, fw.Remote, err = common.ParseForwardPort(farray[1])

		case "remote", "r":
			fw.Mode = "R"
			fw.Local, fw.Remote, err = common.ParseForwardPort(farray[1])

		// other
		default:
			fmt.Fprintf(os.Stderr, "port forward format is incorrect: %s: \"%s\"", server, f)
			continue
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "port forward format is incorrect: %s: \"%s\"", server, f)
			continue
		}

		c.Forwards = append(c.Forwards, fw)
	}

	// append r.PortForward to c.Forwards
	c.Forwards = append(c.Forwards, r.PortForward...)

	return
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
