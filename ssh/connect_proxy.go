package ssh

import (
	"net"

	"github.com/blacknon/lssh/conf"
	"golang.org/x/crypto/ssh"
)

func createProxySshClientViaSsh(proxyConf conf.ServerConfig, proxySshConf *ssh.ClientConfig, proxyClient *ssh.Client) (client *ssh.Client, err error) {
	if proxyClient == nil {
		client, err = ssh.Dial("tcp", net.JoinHostPort(proxyConf.Addr, proxyConf.Port), proxySshConf)
	} else {
		proxyConn, err := proxyClient.Dial("tcp", net.JoinHostPort(proxyConf.Addr, proxyConf.Port))
		if err != nil {
			return client, err
		}

		pConnect, pChans, pReqs, err := ssh.NewClientConn(proxyConn, net.JoinHostPort(proxyConf.Addr, proxyConf.Port), proxySshConf)
		if err != nil {
			return client, err
		}

		client = ssh.NewClient(pConnect, pChans, pReqs)
	}

	net.Dial(network, address)

	return client, err
}
