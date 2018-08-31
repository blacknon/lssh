package ssh

import (
	"net"

	"github.com/blacknon/lssh/conf"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

func createSshClientViaProxy(config conf.ServerConfig, sshConf *ssh.ClientConfig, proxyClient *ssh.Client, dialer proxy.Dialer) (client *ssh.Client, err error) {
	switch {
	// direct connect ssh proxy
	case (proxyClient == nil) && (dialer == nil):
		client, err = ssh.Dial("tcp", net.JoinHostPort(config.Addr, config.Port), sshConf)

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
