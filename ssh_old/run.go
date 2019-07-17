package ssh

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/blacknon/lssh/conf"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

type Run struct {
	ServerList        []string
	Conf              conf.Config
	IsTerm            bool
	IsParallel        bool
	IsShell           bool
	IsX11             bool
	PortForwardLocal  string
	PortForwardRemote string
	ExecCmd           []string
	StdinData         []byte        // 使ってる
	InputData         []byte        // @TODO: Delete???
	OutputData        *bytes.Buffer // use terminal log
	AuthMap           map[AuthKey][]ssh.Signer
}

// Auth map key
type AuthKey struct {
	// auth type:
	//     - agent
	//     - key
	//     - cert
	//     - pkcs11
	Type string

	// auth type value:
	//     - key(path)
	//       ex.) ~/.ssh/id_rsa
	//     - cert(path)
	//       ex.) ~/.ssh/id_rsa.crt
	//     - pkcs11(libpath)
	//       ex.) /usr/local/lib/opensc-pkcs11.so
	Value string
}

const (
	AUTHKEY_AGENT  = "agent"
	AUTHKEY_KEY    = "key"
	AUTHKEY_CERT   = "cert"
	AUTHKEY_PKCS11 = "pkcs11"
)

// Start ssh connect
func (r *Run) Start() {
	// Get stdin data(pipe)
	if runtime.GOOS != "windows" {
		// Windowsではまた別の動作になるかも？？
		// ひとまず、Windowsの場合は無視させてみる
		stdin := 0
		if !terminal.IsTerminal(stdin) {
			r.StdinData, _ = ioutil.ReadAll(os.Stdin)
		}
	}

	// init ssh-agent
	r.SetupSshAgent()

	// create AuthMap
	r.createAuthMap()

	// connect shell
	if len(r.ExecCmd) > 0 { // run command
		r.cmd()
	} else {
		if r.IsShell { // run lssh shell
			r.IsTerm = true
			r.shell()
		} else { // connect remote shell
			r.term()
		}
	}
}

// Create Connect struct array
// (not send ssh packet)
func (r *Run) createConn() (conns []*Connect) {
	for _, server := range r.ServerList {
		c := new(Connect)
		c.Server = server
		c.Conf = r.Conf
		c.IsTerm = r.IsTerm
		c.IsParallel = r.IsParallel
		c.AuthMap = r.AuthMap // @TODO: 特に問題ないだろうが、必要なSignerだけを渡すようにしたほうがいいかも？
		conns = append(conns, c)
	}

	return
}

// print header (select server)
func (r *Run) printSelectServer() {
	serverListStr := strings.Join(r.ServerList, ",")
	fmt.Fprintf(os.Stderr, "Select Server :%s\n", serverListStr)
}

// print header (run command)
func (r *Run) printRunCommand() {
	runCmdStr := strings.Join(r.ExecCmd, " ")
	fmt.Fprintf(os.Stderr, "Run Command   :%s\n", runCmdStr)
}

// print header (port forwarding)
func (r *Run) printPortForward(forwardLocal, forwardRemote string) {
	fmt.Fprintf(os.Stderr, "Port Forward  :local[%s] <=> remote[%s]\n", forwardLocal, forwardRemote)
}

// print header (proxy connect)
func (r *Run) printProxy() {
	if len(r.ServerList) == 1 {
		proxyList := []string{}

		proxyListOrigin, proxyTypeMap, _ := GetProxyList(r.ServerList[0], r.Conf)

		for _, proxy := range proxyListOrigin {
			proxyType := proxyTypeMap[proxy]

			proxyPort := ""
			switch proxyType {
			case "http", "https", "socks5":
				proxyPort = r.Conf.Proxy[proxy].Port
			default:
				proxyPort = r.Conf.Server[proxy].Port
			}

			// "[type://server:port]"
			// ex) [ssh://test-server:22]
			if len(proxyList) == 0 {
				proxyConf := r.Conf.Server[proxy]
				if proxyConf.ProxyCommand != "" {
					proxyCommandStr := "[ProxyCommand:" + proxyConf.ProxyCommand + "]"
					proxyList = append(proxyList, proxyCommandStr)
				} else {
					if proxyPort == "" {
						proxyPort = "22"
					}
					proxyString := "[" + proxyType + "://" + proxy + ":" + proxyPort + "]"
					proxyList = append(proxyList, proxyString)
				}
			}
		}

		serverConf := r.Conf.Server[r.ServerList[0]]
		if len(proxyList) > 0 || serverConf.ProxyCommand != "" {
			if serverConf.ProxyCommand != "" {
				proxyCommandStr := "[ProxyCommand:" + serverConf.ProxyCommand + "]"
				proxyList = []string{proxyCommandStr}
			}

			proxyList = append([]string{"localhost"}, proxyList...)
			proxyList = append(proxyList, r.ServerList[0])
			proxyListStr := strings.Join(proxyList, " => ")
			fmt.Fprintf(os.Stderr, "Proxy         :%s\n", proxyListStr)
		}
	}
}

// runCmdLocal exec command local machine.
func runCmdLocal(cmd string) {
	out, _ := exec.Command("sh", "-c", cmd).CombinedOutput()
	fmt.Printf(string(out))
}

// send input to ssh Session Stdin
func pushInput(isExit <-chan bool, writer io.Writer) {
	rd := bufio.NewReader(os.Stdin)
loop:
	for {
		data, _ := rd.ReadBytes('\n')
		if len(data) > 0 {
			writer.Write(data)
		}

		select {
		case <-isExit:
			break loop
		case <-time.After(10 * time.Millisecond):
			continue
		}
	}
}
