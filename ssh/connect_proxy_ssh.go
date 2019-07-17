package ssh

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/blacknon/lssh/conf"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

// createClientViaProxy return ssh.Client via proxy
func createClientViaProxy(config conf.ServerConfig, sshConf *ssh.ClientConfig, proxyClient *ssh.Client, dialer proxy.Dialer) (client *ssh.Client, err error) {
	switch {
	// direct connect ssh proxy
	case (proxyClient == nil) && (dialer == nil):
		if config.ProxyCommand == "" || config.ProxyCommand == "none" { // not set ProxyCommand
			client, err = ssh.Dial("tcp", net.JoinHostPort(config.Addr, config.Port), sshConf)
		} else { // set ProxyCommand
			client, err = createClientViaProxyCommand(config, sshConf)
		}

	// connect ssh via proxy(http|socks5)
	case (proxyClient == nil) && (dialer != nil):
		proxyConn, err := dialer.Dial("tcp", net.JoinHostPort(config.Addr, config.Port))
		if err != nil {
			return client, err
		}

		pConnect, pChans, pReqs, err := ssh.NewClientConn(proxyConn, net.JoinHostPort(config.Addr, config.Port), sshConf)
		if err != nil {
			return client, err
		}

		client = ssh.NewClient(pConnect, pChans, pReqs)

	// connect ssh via proxy(ssh)
	default:
		if config.ProxyCommand != "" {
			fmt.Fprint(os.Stderr, "`proxy_cmd` takes precedence in `proxy` and `proxy_cmd`\n")
		}

		proxyConn, err := proxyClient.Dial("tcp", net.JoinHostPort(config.Addr, config.Port))
		if err != nil {
			return client, err
		}

		pConnect, pChans, pReqs, err := ssh.NewClientConn(proxyConn, net.JoinHostPort(config.Addr, config.Port), sshConf)
		if err != nil {
			return client, err
		}

		client = ssh.NewClient(pConnect, pChans, pReqs)

	}

	return client, err
}

// createClientViaProxyCommand return ssh.Client via ProxyCommand
func createClientViaProxyCommand(config conf.ServerConfig, sshConf *ssh.ClientConfig) (client *ssh.Client, err error) {
	// set
	proxyCommand := config.ProxyCommand

	// replace variable
	proxyCommand = strings.Replace(proxyCommand, "%h", config.Addr, -1)
	proxyCommand = strings.Replace(proxyCommand, "%p", config.Port, -1)
	proxyCommand = strings.Replace(proxyCommand, "%r", config.User, -1)

	// Create net.Pipe(), and set proxyCommand
	pipeClient, pipeServer := net.Pipe()
	cmd := exec.Command("sh", "-c", proxyCommand)

	// setup FD
	cmd.Stdin = pipeServer
	cmd.Stdout = pipeServer
	cmd.Stderr = os.Stderr

	// run proxyCommand
	if err := cmd.Start(); err != nil {
		return client, err
	}

	// create ssh.Conn
	conn, incomingChannels, incomingRequests, err := ssh.NewClientConn(pipeClient, net.JoinHostPort(config.Addr, config.Port), sshConf)
	if err != nil {
		return client, err
	}

	// create ssh.Client
	client = ssh.NewClient(conn, incomingChannels, incomingRequests)

	return
}
