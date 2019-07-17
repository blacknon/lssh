package ssh

import (
	"fmt"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/conf"
)

// createSshConnect return *sshlib.Connect
// this vaule in ssh.Client with proxy.
func createSshConnect(server string, config conf.Config) (connect *sshlib.Connect, err error) {
	serverConfig := config.Server[server]
}

// proxy struct
type proxyRouteData struct {
	Name string
	Type string
}

// getProxyList return []*pxy function.
//
func getProxyRoute(server string, config conf.Config) (proxyRoute []*proxyRouteData, err error) {
	// conName ... connect server name in loop.
	// conType ... connect server type in loop.
	//             (ssh,http,https,socks,socks5)
	// proxyName ... connect server's proxy name in loop.
	// proxyType ... connect server's proxy type in loop.
	var conName, conType string
	var proxyName, proxyType string

	p := new(proxyRouteData)
	p.Name = server
	p.Type = "ssh"

	proxyRoute = append(proxyRoute, p)

	for {
		isOk := false

		switch conType {
		case "http", "https", "socks", "socks5":
			var conConf conf.ProxyConfig
			conConf, isOk := config.Proxy[conName]
			proxyName = conConf.Proxy
			proxyType = conConf.ProxyType
		case conType == "cmd":
			break
		default:
			var conConf conf.ServerConfig
			conConf, isOk := config.Server[conName]

			if conConf.ProxyCommand != "" || conConf.ProxyCommand != "none" {
				proxyName = conConf.ProxyCommand
				proxyType = "cmd"
			} else {
				proxyName = conConf.Proxy
				proxyType = conConf.ProxyType
			}
		}

		// not use pre proxy
		if proxyName == "" {
			break
		}

		if !isOk {
			err = fmt.Errorf("Not Found proxy : %s", server)
			return nil, nil, err
		}

		p := new(proxyRouteData)
		p.Name = proxyName
		switch proxyType {
		case "http", "https", "socks", "socks5":
			p.Type = proxyType
		default:
			p.Type = "ssh"
		}

		proxyList = append(proxyList, p)
		proxyType[proxy.Name] = proxy.Type

		conName = proxyName
		conType = proxyType
	}

	// reverse proxyServers slice
	for i, j := 0, len(proxyList)-1; i < j; i, j = i+1, j-1 {
		proxyList[i], proxyList[j] = proxyList[j], proxyList[i]
	}

	return
}
