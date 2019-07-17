package ssh

import (
	"bytes"
	"fmt"
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

	// PortForwarding
	PortForwardLocal  string
	PortForwardRemote string

	// Exec command
	ExecCmd []string

	// Agent is ssh-agent.
	// In agent.Agent or agent.ExtendedAgent.
	agent interace

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
	// Get stdin data(pipe)
	if runtime.GOOS != "windows" {
		stdin := 0
		if !terminal.IsTerminal(stdin) {
			r.StdinData, _ = ioutil.ReadAll(os.Stdin)
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
	// server config
	server := r.ServerList[0]
	config := r.Conf.Server[server]

	// Craete sshlib.Connect (Connect Proxy loop)
	connect, err := r.createSshConnect(server)

	// x11 forwarding
	if config.X11 {
		connect.ForwardX11 = true
	}

	// ssh-agent
	connect.ConnectSshAgent()
	if config.SSHAgentUse {
		connect.Agent = r.Agent
		connect.ForwardSshAgent(session)
	}

	// Port Forwarding
	if r.PortForwardLocal != "" && r.PortForwardRemote != "" {
		err := connect.TCPForward(r.PortForwardLocal, r.PortForwardRemote)
		if err != nil {
			fmt.Println(err)
		}
	}

	// Connect shell
	connect.Shell()
}

//
func createSshConnect(server string, config conf.Config) (connect *sshlib.Connect, err error) {
	serverConfig := conf.ServerConfig[server]

}

// getProxyList return proxy list and map by proxy type.
func getProxyList(server string, config conf.Config) (proxyList []string, proxyType map[string]string, err error) {
	var targetType string
	var preProxy, preProxyType string

	targetServer := server
	proxyType = map[string]string{}

	for {
		isOk := false

		switch targetType {
		case "http", "https", "socks5":
			preProxyConf, isOk := config.Proxy[targetServer]
			preProxy = preProxyConf.Proxy
			preProxyType = preProxyConf.ProxyType

		default:
			var preProxyConf conf.ServerConfig
			preProxyConf, isOk = config.Server[targetServer]
			preProxy = preProxyConf.Proxy
			preProxyType = preProxyConf.ProxyType
		}

		// not use pre proxy
		if preProxy == "" {
			break
		}

		if !isOk {
			err = fmt.Errorf("Not Found proxy : %s", targetServer)
			return nil, nil, err
		}

		// set proxy info
		proxy := new(Proxy)
		proxy.Name = preProxy

		switch preProxyType {
		case "http", "https", "socks5":
			proxy.Type = preProxyType
		default:
			proxy.Type = "ssh"
		}

		proxyList = append(proxyList, proxy.Name)
		proxyType[proxy.Name] = proxy.Type

		targetServer = proxy.Name
		targetType = proxy.Type
	}

	// reverse proxyServers slice
	for i, j := 0, len(proxyList)-1; i < j; i, j = i+1, j-1 {
		proxyList[i], proxyList[j] = proxyList[j], proxyList[i]
	}

	return
}
