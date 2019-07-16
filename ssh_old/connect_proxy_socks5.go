package ssh

import (
	"net"

	"github.com/blacknon/lssh/conf"
	"golang.org/x/net/proxy"
)

// createProxyDialerSocks5 return proxy.Dialer via Socks5 proxy
func createProxyDialerSocks5(proxyConf conf.ProxyConfig) (proxyDialer proxy.Dialer, err error) {
	var proxyAuth *proxy.Auth

	if proxyConf.User != "" && proxyConf.Pass != "" {
		proxyAuth.User = proxyConf.User
		proxyAuth.Password = proxyConf.Pass
	}

	proxyDialer, err = proxy.SOCKS5("tcp", net.JoinHostPort(proxyConf.Addr, proxyConf.Port), proxyAuth, proxy.Direct)
	return
}
