// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// TODO: CheckKnownHostsを使ってなかったので、HostKeyのチェックを追加し、さらにオプションで状況に応じた挙動の変化を実装する。
// TODO: HostKey Checkの挙動をどうするかについて、Configで指定できるようにする

package ssh

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"os"
	osuser "os/user"
	"strings"
	"time"

	"github.com/blacknon/go-sshlib"
	conf "github.com/blacknon/lssh/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

// createSshConnectWithControlPersist constructs a Connect that passes
// ProxyRoute directly to go-sshlib. It builds ControlPersist-capable
// auth methods from server config (passwords, keys, certs, pkcs11).
func (r *Run) createSshConnectWithControlPersist(server string, proxyRoute []*proxyRouteData) (connect *sshlib.Connect, err error) {
	// build sshlib.ProxyRoute list from proxyRoute (internal)
	routes := make([]sshlib.ProxyRoute, 0, len(proxyRoute))
	for _, p := range proxyRoute {
		switch p.Type {
		case "http", "https", "socks", "socks5":
			c := r.Conf.Proxy[p.Name]
			routes = append(routes, sshlib.ProxyRoute{
				Type:     p.Type,
				Addr:     c.Addr,
				Port:     c.Port,
				User:     c.User,
				Password: c.Pass,
			})
		case "command":
			routes = append(routes, sshlib.ProxyRoute{
				Type:    "command",
				Command: p.Name,
			})
		default:
			// ssh hop: build ControlPersistAuth from config (not runtime auth)
			c := r.Conf.Server[p.Name]
			methods, err := r.buildControlPersistAuthMethodsFromConfig(p.Name, c)
			if err != nil {
				return nil, err
			}
			routes = append(routes, sshlib.ProxyRoute{
				Type: "ssh",
				Addr: c.Addr,
				Port: p.Port,
				User: c.User,
				Auth: &sshlib.ControlPersistAuth{AuthMethods: methods},
			})
		}
	}

	// target server config
	s := r.effectiveServerConfig(server, false)

	// build top-level control-persist auth methods from config
	topMethods, err := r.buildControlPersistAuthMethodsFromConfig(server, s)
	if err != nil {
		return nil, err
	}

	connect = &sshlib.Connect{
		ProxyRoute:            routes,
		ForwardAgent:          s.SSHAgentUse,
		Agent:                 r.agent,
		ForwardX11:            s.X11 || r.X11,
		ForwardX11Trusted:     s.X11Trusted || r.X11Trusted,
		TTY:                   r.IsTerm,
		ConnectTimeout:        s.ConnectTimeout,
		SendKeepAliveMax:      s.ServerAliveCountMax,
		SendKeepAliveInterval: s.ServerAliveCountInterval,
		CheckKnownHosts:       s.CheckKnownHosts,
		KnownHostsFiles:       s.KnownHostsFiles,
		OverwriteKnownHosts:   true,
		ControlPersistAuth:    &sshlib.ControlPersistAuth{AuthMethods: topMethods},
	}

	if s.ControlMaster {
		connect.ControlMaster = "auto"
	} else {
		connect.ControlMaster = "no"
	}

	connect.ControlPath = expandControlPath(s.ControlPath, server, s)

	// ControlPersist duration
	if s.ControlPersist > 0 {
		// s.ControlPersist is seconds in config type
		connect.ControlPersist, _ = time.ParseDuration(fmt.Sprintf("%ds", s.ControlPersist))
	}

	// done; return without calling CreateClient here — caller will call CreateClient
	return connect, nil
}

// buildControlPersistAuthMethodsFromConfig creates ssh.AuthMethod values suitable
// for ControlPersist from raw ServerConfig. It avoids using runtime agent/pkcs11
// objects when possible; for PKCS11 it prompts for PIN if absent.
func (r *Run) buildControlPersistAuthMethodsFromConfig(name string, c conf.ServerConfig) (methods []ssh.AuthMethod, err error) {
	methods = []ssh.AuthMethod{}

	// password
	if c.Pass != "" || c.PassRef != "" {
		pass, err := r.resolveLiteralOrRef(name, "pass", c.Pass, c.PassRef)
		if err != nil {
			return nil, err
		}
		if pass != "" {
			methods = append(methods, sshlib.CreateAuthMethodPassword(pass))
		}
	}
	for _, p := range c.Passes {
		methods = append(methods, sshlib.CreateAuthMethodPassword(p))
	}

	// single key
	if c.Key != "" || c.KeyRef != "" {
		am, err := r.createPublicKeyAuthMethod(name, c)
		if err != nil {
			return nil, err
		}
		methods = append(methods, am)
	}
	// multiple keys
	for _, key := range c.Keys {
		pair := strings.SplitN(key, "::", 2)
		keyName := pair[0]
		keyPass := ""
		if len(pair) > 1 {
			keyPass = pair[1]
		}
		am, err := sshlib.CreateAuthMethodPublicKey(keyName, keyPass)
		if err != nil {
			return nil, err
		}
		methods = append(methods, am)
	}

	// certificate
	if c.Cert != "" || c.CertRef != "" {
		am, err := r.createCertificateAuthMethod(name, c)
		if err != nil {
			return nil, err
		}
		methods = append(methods, am)
	}
	for _, cert := range c.Certs {
		pair := strings.SplitN(cert, "::", 3)
		if len(pair) < 2 {
			continue
		}
		certName := pair[0]
		keyName := pair[1]
		keyPass := ""
		if len(pair) > 2 {
			keyPass = pair[2]
		}
		signer, err := sshlib.CreateSignerPublicKeyPrompt(keyName, keyPass)
		if err != nil {
			return nil, err
		}
		am, err := sshlib.CreateAuthMethodCertificate(certName, signer)
		if err != nil {
			return nil, err
		}
		methods = append(methods, am)
	}

	// PKCS11: prompt for PIN if needed and create auth methods
	if c.PKCS11Use {
		pin, err := r.resolveLiteralOrRef(name, "pkcs11pin", c.PKCS11PIN, c.PKCS11PINRef)
		if err != nil {
			return nil, err
		}
		if pin == "" {
			// Prompt user for PIN (simple IPC via stdin)
			fmt.Fprintf(os.Stderr, "PKCS11 PIN for provider %s: ", c.PKCS11Provider)
			reader := bufio.NewReader(os.Stdin)
			line, _ := reader.ReadString('\n')
			pin = strings.TrimSpace(line)
		}
		pkMethods, err := sshlib.CreateAuthMethodPKCS11(c.PKCS11Provider, pin)
		if err != nil {
			return nil, err
		}
		methods = append(methods, pkMethods...)
	}

	return methods, nil
}

// CreateSshConnect return *sshlib.Connect
// this vaule in ssh.Client with proxy.
func (r *Run) CreateSshConnect(server string) (connect *sshlib.Connect, err error) {
	return r.createSshConnect(server, false)
}

// CreateSshConnectDirect returns *sshlib.Connect with ControlMaster/ControlPersist disabled.
// This is used by features that require a concrete *ssh.Client (e.g. SFTP clients).
func (r *Run) CreateSshConnectDirect(server string) (connect *sshlib.Connect, err error) {
	return r.createSshConnect(server, true)
}

func (r *Run) effectiveServerConfig(server string, forceDirect bool) conf.ServerConfig {
	s := r.Conf.Server[server]

	if r.ControlMasterOverride != nil {
		s.ControlMaster = *r.ControlMasterOverride
		if !s.ControlMaster {
			s.ControlPersist = 0
		}
	}

	if forceDirect {
		s.ControlMaster = false
		s.ControlPersist = 0
	}

	return s
}

// createSshConnect is the main function for creating sshlib.Connect objects.
// If forceDirect is true, it disables ControlMaster/ControlPersist for the connection (used for features like SFTP that require a concrete *ssh.Client).
func (r *Run) createSshConnect(server string, forceDirect bool) (connect *sshlib.Connect, err error) {
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
	var dialer sshlib.ProxyDialer
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
	s := r.effectiveServerConfig(server, forceDirect)

	// ControlPersist path is only valid when ControlMaster is enabled.
	if s.ControlMaster && s.ControlPersist > 0 {
		connect, err = r.createSshConnectWithControlPersist(server, proxyRoute)
		if err != nil {
			return nil, err
		}

		if r.EnableStdoutMutex {
			connect.StdoutMutex = &r.stdoutMutex
		}

		err = connect.CreateClient(s.Addr, s.Port, s.User, r.serverAuthMethodMap[server])
		if err != nil {
			if client, ok := dialer.(*ssh.Client); ok {
				client.Close()
			}
			return nil, err
		}

		// Setup tunnel if requested
		if r.TunnelEnabled {
			tun, terr := connect.Tunnel(r.TunnelLocal, r.TunnelRemote)
			if terr != nil {
				if client, ok := dialer.(*ssh.Client); ok {
					client.Close()
				}
				return nil, fmt.Errorf("tunnel error: %w", terr)
			}
			r.ActiveTunnel = tun
			go func() {
				_ = tun.Wait()
			}()
		}

		return connect, nil
	}

	// set x11
	var x11 bool
	if s.X11 || r.X11 {
		x11 = true
	}

	// set X11Trusted
	var x11Trusted bool
	if s.X11Trusted || r.X11Trusted {
		x11Trusted = true
	}

	// connect target server
	connect = &sshlib.Connect{
		ProxyDialer:           dialer,
		ForwardAgent:          s.SSHAgentUse,
		Agent:                 r.agent,
		ForwardX11:            x11,
		ForwardX11Trusted:     x11Trusted,
		TTY:                   r.IsTerm,
		ConnectTimeout:        s.ConnectTimeout,
		SendKeepAliveMax:      s.ServerAliveCountMax,
		SendKeepAliveInterval: s.ServerAliveCountInterval,
		CheckKnownHosts:       s.CheckKnownHosts,
		KnownHostsFiles:       s.KnownHostsFiles,
		OverwriteKnownHosts:   true,
	}

	if s.ControlMaster {
		connect.ControlMaster = "auto"
	} else {
		connect.ControlMaster = "no"
	}

	connect.ControlPath = expandControlPath(s.ControlPath, server, s)

	persist, err := time.ParseDuration(fmt.Sprintf("%ds", s.ControlPersist))
	if err != nil || persist <= 0 {
		persist = 10 * time.Minute
	}
	connect.ControlPersist = persist

	if r.EnableStdoutMutex {
		connect.StdoutMutex = &r.stdoutMutex
	}

	err = connect.CreateClient(s.Addr, s.Port, s.User, r.serverAuthMethodMap[server])
	if err != nil {
		if client, ok := dialer.(*ssh.Client); ok {
			client.Close()
		}

		return nil, err
	}

	// Setup tunnel if requested
	if r.TunnelEnabled {
		tun, terr := connect.Tunnel(r.TunnelLocal, r.TunnelRemote)
		if terr != nil {
			if client, ok := dialer.(*ssh.Client); ok {
				client.Close()
			}
			return nil, fmt.Errorf("tunnel error: %w", terr)
		}
		r.ActiveTunnel = tun
		go func() {
			_ = tun.Wait()
		}()
	}

	return connect, nil
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
			if proxyConf, ok := config.Proxy[proxyName]; ok {
				p.Port = proxyConf.Port
			} else {
				p.Port = proxyPort
			}
		case "command":
			p.Type = proxyType
		default:
			p.Type = "ssh"
			if serverConf, ok := config.Server[proxyName]; ok {
				p.Port = serverConf.Port
			} else {
				p.Port = proxyPort
			}
		}

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

func expandControlPath(controlPath, server string, config conf.ServerConfig) string {
	localHost, _ := os.Hostname()
	localShortHost := localHost
	if idx := strings.IndexByte(localShortHost, '.'); idx >= 0 {
		localShortHost = localShortHost[:idx]
	}

	localUser := ""
	homeDir := ""
	if currentUser, err := osuser.Current(); err == nil {
		localUser = currentUser.Username
		homeDir = currentUser.HomeDir
	}

	controlHash := fmt.Sprintf("%x", sha1.Sum([]byte(localHost+config.Addr+config.Port+config.User)))

	replacer := strings.NewReplacer(
		"%%", "%",
		"%C", controlHash,
		"%d", homeDir,
		"%h", config.Addr,
		"%L", localShortHost,
		"%l", localHost,
		"%n", server,
		"%p", config.Port,
		"%r", config.User,
		"%u", localUser,
	)

	return replacer.Replace(controlPath)
}

func expansionProxyCommand(proxyCommand string, config conf.ServerConfig) string {
	// replace variable
	proxyCommand = strings.Replace(proxyCommand, "%h", config.Addr, -1)
	proxyCommand = strings.Replace(proxyCommand, "%p", config.Port, -1)
	proxyCommand = strings.Replace(proxyCommand, "%r", config.User, -1)

	return proxyCommand
}
