package ssh

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"syscall"

	"github.com/blacknon/lssh/conf"
	"golang.org/x/crypto/ssh/terminal"
)

type Run struct {
	ServerList        []string
	Conf              conf.Config
	IsTerm            bool
	IsParallel        bool
	PortForwardLocal  string
	PortForwardRemote string
	ExecCmd           []string
	StdinData         []byte
	InputData         []byte // @TODO: Delete???
	OutputData        *bytes.Buffer
}

func (r *Run) Start() {
	// Get stdin data(pipe)
	if !terminal.IsTerminal(syscall.Stdin) {
		r.StdinData, _ = ioutil.ReadAll(os.Stdin)
	}

	// @TODO: r.shell()で分岐するよう、selectにする
	if len(r.ExecCmd) > 0 {
		r.cmd()
	} else {
		r.term()
	}
}

func (r *Run) printSelectServer() {
	serverListStr := strings.Join(r.ServerList, ",")
	fmt.Fprintf(os.Stderr, "Select Server :%s\n", serverListStr)
}

func (r *Run) printRunCommand() {
	runCmdStr := strings.Join(r.ExecCmd, " ")
	fmt.Fprintf(os.Stderr, "Run Command   :%s\n", runCmdStr)
}

func (r *Run) printPortForward(forwardLocal, forwardRemote string) {
	fmt.Fprintf(os.Stderr, "Port Forward  :local[%s] <=> remote[%s]\n", forwardLocal, forwardRemote)
}

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
			proxyString := "[" + proxyType + "://" + proxy + ":" + proxyPort + "]"
			proxyList = append(proxyList, proxyString)
		}

		if len(proxyList) > 0 {
			proxyList = append([]string{"localhost"}, proxyList...)
			proxyList = append(proxyList, r.ServerList[0])
			proxyListStr := strings.Join(proxyList, " => ")
			fmt.Fprintf(os.Stderr, "Proxy         :%s\n", proxyListStr)
		}
	}
}
