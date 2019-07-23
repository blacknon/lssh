package ssh

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/blacknon/lssh/conf"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

type Run struct {
	ServerList []string
	Conf       conf.Config

	// Mode value in
	//     - shell
	//     - cmd
	//     - pshell
	Mode string

	// tty use
	IsTerm bool

	// parallel connect
	IsParallel bool

	// x11 forwarding
	X11 bool

	// use or not-use local bashrc.
	// IsNotBashrc takes precedence.
	IsBashrc    bool
	IsNotBashrc bool

	// PortForwarding
	PortForwardLocal  string
	PortForwardRemote string

	// Exec command
	ExecCmd []string

	// Agent is ssh-agent.
	// In agent.Agent or agent.ExtendedAgent.
	agent interface{}

	// StdinData from pipe
	stdinData []byte

	// use terminal log
	outputData *bytes.Buffer

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
	if runtime.GOOS != "windows" {
		stdin := 0
		if !terminal.IsTerminal(stdin) {
			r.stdinData, _ = ioutil.ReadAll(os.Stdin)
		}
	}

	// Set portforward mode
	// TODO(blacknon): 追加。基本はLocalで、Remoteが指定されてる場合は上書き(順番は←の順でチェックすれば上書きされるはず)。

	// create AuthMap
	r.createAuthMethodMap()

	// connect shell
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
func (r *Run) printPortForward(forwardLocal, forwardRemote string) {
	if forwardLocal != "" && forwardRemote != "" {
		fmt.Fprintf(os.Stderr, "Port Forward  :local[%s] <=> remote[%s]\n", forwardLocal, forwardRemote)
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
		str := "[" + pxy.Type + "://" + pxy.Name
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
func runCmdLocal(cmd string) {
	out, _ := exec.Command("sh", "-c", cmd).CombinedOutput()
	fmt.Printf(string(out))
}
