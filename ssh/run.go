package ssh

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"

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
	//     - lsshshell
	Mode string

	// tty use
	IsTerm bool

	// parallel connect
	IsParallel bool

	// x11 forwarding
	X11 bool

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

	// create AuthMap
	r.createAuthMethodMap()

	// connect shell
	switch {
	case len(r.ExecCmd) > 0 && r.Mode == "cmd":
		// connect and run command
		r.cmd()

	case r.Mode == "shell":
		// connect remote shell
		err = r.shell()

	case r.Mode == "lsshshell":
		// start lsshshell
		r.lsshShell()

	default:
		return
	}

	if err != nil {
		fmt.Println(err)
	}
}

func (r *Run) cmd() {
	fmt.Println("now working...")
}

func (r *Run) lsshShell() {
	fmt.Println("now working...")
}
