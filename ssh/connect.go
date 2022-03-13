// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"fmt"
	"strings"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/conf"
	"golang.org/x/net/proxy"
)

// CreateSshConnect return *sshlib.Connect
// this vaule in ssh.Client with proxy.
func (r *Run) CreateSshConnect(server string) (connect *sshlib.Connect, err error) {
	// create proxyRoute
	proxyRoute, err := getProxyRoute(server, r.Conf)
	if err != nil {
		return
	}

	// Connect ssh-agent
	if r.agent == nil {
		r.agent = sshlib.ConnectSshAgent()
	}

	// setup dialer
	var dialer proxy.Dialer
	dialer = proxy.Direct

	// Connect loop proxy server
	for _, p := range proxyRoute {
		config := r.Conf

		switch p.Type {
		case "http", "https", "socks", "socks5":
			c := config.Proxy[p.Name]

			pxy := &sshlib.Proxy{
				Type:      p.Type,
				Forwarder: dialer,
				Addr:      c.Addr,
				Port:      c.Port,
				User:      c.User,
				Password:  c.Pass,
			}
			dialer, err = pxy.CreateProxyDialer()
		case "command":
			pxy := &sshlib.Proxy{
				Type:    p.Type,
				Command: p.Name,
			}
			dialer, err = pxy.CreateProxyDialer()
		default:
			c := config.Server[p.Name]
			pxy := &sshlib.Connect{
				ProxyDialer: dialer,
			}
			err := pxy.CreateClient(c.Addr, c.Port, c.User, r.serverAuthMethodMap[p.Name])
			if err != nil {
				return connect, err
			}

			dialer = pxy.Client
		}
	}

	// server conf
	s := r.Conf.Server[server]

	// set x11
	var x11 bool
	if s.X11 || r.X11 {
		x11 = true
	}

	// connect target server
	connect = &sshlib.Connect{
		ProxyDialer:           dialer,
		ForwardAgent:          s.SSHAgentUse,
		Agent:                 r.agent,
		ForwardX11:            x11,
		TTY:                   r.IsTerm,
		ConnectTimeout:        s.ConnectTimeout,
		SendKeepAliveMax:      s.ServerAliveCountMax,
		SendKeepAliveInterval: s.ServerAliveCountInterval,
	}

	err = connect.CreateClient(s.Addr, s.Port, s.User, r.serverAuthMethodMap[server])

	return
}

// proxy struct
type proxyRouteData struct {
	Name string
	Type string
	Port string
}

// getProxyList return []*pxy function.
func getProxyRoute(server string, config conf.Config) (proxyRoute []*proxyRouteData, err error) {
	var conName, conType string
	var proxyName, proxyType, proxyPort string
	var isOk bool

	conName = server
	conType = "ssh"

proxyLoop:
	for {
		switch conType {
		case "http", "https", "socks", "socks5":
			var conConf conf.ProxyConfig
			conConf, isOk = config.Proxy[conName]
			proxyName = conConf.Proxy
			proxyType = conConf.ProxyType
			proxyPort = conConf.Port
		case "command":
			break proxyLoop
		default:
			var conConf conf.ServerConfig
			conConf, isOk = config.Server[conName]

			// If ProxyCommand is set, give priority to that
			if conConf.ProxyCommand != "" && conConf.ProxyCommand != "none" {

				proxyName = expansionProxyCommand(conConf.ProxyCommand, conConf)
				proxyType = "command"
				proxyPort = ""
			} else {
				proxyName = conConf.Proxy
				proxyType = conConf.ProxyType
				proxyPort = conConf.Port
			}
		}

		// not use proxy
		if proxyName == "" {
			break
		}

		//
		if !isOk {
			err = fmt.Errorf("Not Found proxy : %s", server)
			return nil, err
		}

		//
		p := new(proxyRouteData)
		p.Name = proxyName
		switch proxyType {
		case "http", "https", "socks", "socks5":
			p.Type = proxyType
		case "command":
			p.Type = proxyType
		default:
			p.Type = "ssh"
		}
		p.Port = proxyPort

		proxyRoute = append(proxyRoute, p)
		conName = proxyName
		conType = proxyType
	}

	// reverse proxy slice
	for i, j := 0, len(proxyRoute)-1; i < j; i, j = i+1, j-1 {
		proxyRoute[i], proxyRoute[j] = proxyRoute[j], proxyRoute[i]
	}

	return
}

func expansionProxyCommand(proxyCommand string, config conf.ServerConfig) string {
	// replace variable
	proxyCommand = strings.Replace(proxyCommand, "%h", config.Addr, -1)
	proxyCommand = strings.Replace(proxyCommand, "%p", config.Port, -1)
	proxyCommand = strings.Replace(proxyCommand, "%r", config.User, -1)

	return proxyCommand
}
