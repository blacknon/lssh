package ssh

import (
	"bytes"
	"io/ioutil"
	"os"
	"runtime"

	"github.com/blacknon/go-sshlib"
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

	// use x11 forwarding
	IsX11 bool

	// PortForwarding
	PortForwardLocal  string
	PortForwardRemote string

	// Exec command
	ExecCmd []string

	// Stdin data
	StdinData  []byte        // 使ってる
	OutputData *bytes.Buffer // use terminal log

	// Signer map
	AuthMethodMap map[AuthKey][]ssh.AuthMethod
}

// Auth map key
type AuthKey struct {
	// auth type:
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
	AUTHKEY_KEY      = "key"
	AUTHKEY_CERT     = "cert"
	AUTHKEY_PKCS11   = "pkcs11"
)

// Start ssh connect
func (r *Run) Start() {
	// Get stdin data(pipe)
	if runtime.GOOS != "windows" {
		stdin := 0
		if !terminal.IsTerminal(stdin) {
			r.StdinData, _ = ioutil.ReadAll(os.Stdin)
		}
	}

	// create AuthMap
	r.createAuthMap()

	// connect shell
	switch {
	case len(r.ExecCmd) > 0 && r.Mode == "cmd":
		// connect and run command
		r.cmd()

	case r.Mode == "shell":
		// connect remote shell
		r.shell()

	case r.Mode == "lsshshell":
		// start lsshshell
		r.lsshShell()

	default:
		return
	}
}

func (r *Run) cmd() {
	connect := &sshlib.Connect{}

	if r.IsX11 {
		connect.ForwardX11 = true
	}

}

func (r *Run) shell() {
	connect := &sshlib.Connect{}

	if r.IsX11 {
		connect.ForwardX11 = true
	}

}
