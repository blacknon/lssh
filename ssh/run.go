package ssh

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/blacknon/lssh/conf"
	"golang.org/x/crypto/ssh/terminal"
)

type Run struct {
	ServerList        []string
	Conf              conf.Config
	IsTerm            bool
	IsParallel        bool
	IsShell           bool
	PortForwardLocal  string
	PortForwardRemote string
	ExecCmd           []string
	StdinData         []byte
	InputData         []byte        // @TODO: Delete???
	OutputData        *bytes.Buffer // use terminal log
}

func (r *Run) Start() {
	// Get stdin data(pipe)
	if !terminal.IsTerminal(syscall.Stdin) {
		r.StdinData, _ = ioutil.ReadAll(os.Stdin)
	}

	// connect shell
	if len(r.ExecCmd) > 0 { // run command
		r.cmd()
	} else {
		if r.IsShell { // run lssh shell
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

func outColorStrings(num int, inStrings string) (str string) {
	// 1=Red,2=Yellow,3=Blue,4=Magenta,0=Cyan
	color := 31 + num%5

	str = fmt.Sprintf("\x1b[%dm%s\x1b[0m", color, inStrings)
	return
}
