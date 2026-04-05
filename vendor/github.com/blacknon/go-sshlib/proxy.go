// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os/exec"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

type ProxyDialer interface {
	Dial(network, addr string) (net.Conn, error)
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

type ProxyRoute struct {
	Type string

	Addr string
	Port string

	User     string
	Password string

	Command string

	Auth *ControlPersistAuth
}

type ContextDialer struct {
	Dialer proxy.Dialer
}

func (c *ContextDialer) GetDialer() proxy.Dialer {
	return c.Dialer
}

func (c *ContextDialer) Dial(network, addr string) (net.Conn, error) {
	return c.Dialer.Dial(network, addr)
}

func (c *ContextDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	// Simply call the DialContext method if supported
	if dialerCtx, ok := c.Dialer.(interface {
		DialContext(context.Context, string, string) (net.Conn, error)
	}); ok {
		return dialerCtx.DialContext(ctx, network, addr)
	}

	// Fallback if DialContext is not supported
	connChan := make(chan net.Conn, 1)
	errChan := make(chan error, 1)

	go func() {
		conn, err := c.Dialer.Dial(network, addr)
		if err != nil {
			errChan <- err
			return
		}
		connChan <- conn
	}()

	select {
	case conn := <-connChan:
		return conn, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type Proxy struct {
	// Type set proxy type.
	// Can specify `http`, `https`, `socks`, `socks5`, `command`.
	//
	// It is read at the time of specification depending on the type.
	Type string

	// Addr set proxy address.
	//
	Addr string

	// Port set proxy port.
	//
	Port string

	// Port set proxy user.
	//
	User string

	// Port set proxy user.
	//
	Password string

	// Command only use Type `command`.
	//
	Command string

	// Forwarder set Dialer.
	Forwarder ProxyDialer
}

type controlPersistProxyRoute struct {
	Type string

	Addr string
	Port string

	User     string
	Password string

	Command string

	Auth []controlPersistAuthMethodDefinition
}

// CreateProxyDialer retrun ProxyDialer.
func (p *Proxy) CreateProxyDialer() (proxyContextDialer ProxyDialer, err error) {
	var proxyDialer proxy.Dialer
	switch p.Type {
	case "http", "https":
		proxyDialer, err = p.CreateHttpProxyDialer()
	case "socks", "socks5":
		proxyDialer, err = p.CreateSocks5ProxyDialer()
	case "command":
		proxyDialer, err = p.CreateProxyCommandProxyDialer()
	}

	proxyContextDialer = &ContextDialer{Dialer: proxyDialer}

	return
}

// CreateHttpProxy return ProxyDialer as http proxy.
func (p *Proxy) CreateHttpProxyDialer() (proxyDialer proxy.Dialer, err error) {
	// Regist dialer
	proxy.RegisterDialerType("http", newHttpProxy)
	proxy.RegisterDialerType("https", newHttpProxy)

	proxyURL := p.Type + "://" + p.Addr
	if p.User != "" && p.Password != "" {
		proxyURL = p.Type + "://" + p.User + ":" + p.Password + "@" + p.Addr
	}

	if p.Port != "" {
		proxyURL = proxyURL + ":" + string(p.Port)
	}

	proxyURI, _ := url.Parse(proxyURL)

	var forwarder proxy.Dialer
	forwarder = proxy.Direct
	if p.Forwarder != nil {
		forwarder = p.Forwarder
	}
	proxyDialer, err = proxy.FromURL(proxyURI, forwarder)

	return
}

// CreateSocks5Proxy return ProxyDialer as Socks5 proxy.
func (p *Proxy) CreateSocks5ProxyDialer() (proxyDialer proxy.Dialer, err error) {
	var proxyAuth *proxy.Auth

	if p.User != "" && p.Password != "" {
		proxyAuth = &proxy.Auth{}
		proxyAuth.User = p.User
		proxyAuth.Password = p.Password
	}

	var forwarder ProxyDialer
	forwarder = proxy.Direct
	if p.Forwarder != nil {
		forwarder = p.Forwarder
	}

	return proxy.SOCKS5("tcp", net.JoinHostPort(p.Addr, p.Port), proxyAuth, forwarder)
}

// CreateProxyCommandProxyDialer as ProxyCommand.
// When passing ProxyCommand, replace %h, %p and %r etc...
func (p *Proxy) CreateProxyCommandProxyDialer() (proxyDialer proxy.Dialer, err error) {
	np := new(NetPipe)
	np.Command = p.Command
	proxyDialer = np

	return
}

type NetPipe struct {
	Command string
	ctx     context.Context
	Cmd     *exec.Cmd
}

func (n *NetPipe) Dial(network, addr string) (con net.Conn, err error) {
	network = ""
	addr = ""

	// Create net.Pipe(), and set proxyCommand
	con, srv := net.Pipe()

	n.Cmd = exec.Command("sh", "-c", n.Command)

	// setup FD
	n.Cmd.Stdin = srv
	n.Cmd.Stdout = srv
	n.Cmd.Stderr = log.Writer()

	// Start the command
	err = n.Cmd.Start()

	// Close the write end of the pipe
	go func() {
		n.Cmd.Wait()
		srv.Close()
	}()

	return
}

func (n *NetPipe) DialContext(ctx context.Context, network, addr string) (con net.Conn, err error) {
	connChan := make(chan net.Conn, 1)
	errChan := make(chan error, 1)

	go func() {
		conn, err := n.Dial(network, addr)
		if err != nil {
			errChan <- err
			return
		}

		connChan <- conn
	}()

	select {
	case conn := <-connChan:
		return conn, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		if n.Cmd != nil && n.Cmd.Process != nil {
			_ = n.Cmd.Process.Kill()
		}
		return nil, ctx.Err()
	}
}

func buildProxyRouteDialer(routes []ProxyRoute, prompt PromptFunc) (proxy.ContextDialer, []*Connect, error) {
	var current ProxyDialer = proxy.Direct
	var proxyConnects []*Connect

	for i, route := range routes {
		switch route.Type {
		case "http", "https", "socks", "socks5", "command":
			proxyDialer, err := (&Proxy{
				Type:      route.Type,
				Addr:      route.Addr,
				Port:      route.Port,
				User:      route.User,
				Password:  route.Password,
				Command:   route.Command,
				Forwarder: current,
			}).CreateProxyDialer()
			if err != nil {
				closeProxyConnectList(proxyConnects)
				return nil, nil, err
			}
			if _, ok := proxyDialer.(proxy.ContextDialer); !ok {
				closeProxyConnectList(proxyConnects)
				return nil, nil, fmt.Errorf("sshlib: proxy route[%d] does not implement proxy.ContextDialer", i)
			}
			current = proxyDialer
		case "ssh":
			authMethods, err := route.authMethods(prompt)
			if err != nil {
				closeProxyConnectList(proxyConnects)
				return nil, nil, err
			}

			con := &Connect{
				ProxyDialer: current,
			}
			if err := con.CreateClient(route.Addr, route.portOrDefault(), route.User, authMethods); err != nil {
				closeProxyConnectList(proxyConnects)
				return nil, nil, err
			}

			current = con.Client
			proxyConnects = append(proxyConnects, con)
		default:
			closeProxyConnectList(proxyConnects)
			return nil, nil, fmt.Errorf("sshlib: unsupported proxy route type %q at index %d", route.Type, i)
		}
	}

	contextDialer, ok := current.(proxy.ContextDialer)
	if !ok {
		closeProxyConnectList(proxyConnects)
		return nil, nil, errors.New("sshlib: final proxy route dialer does not implement proxy.ContextDialer")
	}

	return contextDialer, proxyConnects, nil
}

func buildControlPersistProxyRouteDialer(routes []controlPersistProxyRoute, prompt PromptFunc) (proxy.ContextDialer, []*Connect, error) {
	var current ProxyDialer = proxy.Direct
	var proxyConnects []*Connect

	for i, route := range routes {
		switch route.Type {
		case "http", "https", "socks", "socks5", "command":
			proxyDialer, err := (&Proxy{
				Type:      route.Type,
				Addr:      route.Addr,
				Port:      route.Port,
				User:      route.User,
				Password:  route.Password,
				Command:   route.Command,
				Forwarder: current,
			}).CreateProxyDialer()
			if err != nil {
				closeProxyConnectList(proxyConnects)
				return nil, nil, err
			}
			if _, ok := proxyDialer.(proxy.ContextDialer); !ok {
				closeProxyConnectList(proxyConnects)
				return nil, nil, fmt.Errorf("sshlib: control persist proxy route[%d] does not implement proxy.ContextDialer", i)
			}
			current = proxyDialer
		case "ssh":
			authMethods, err := createControlPersistAuthMethodsWithPrompt(route.Auth, prompt)
			if err != nil {
				closeProxyConnectList(proxyConnects)
				return nil, nil, err
			}

			con := &Connect{
				ProxyDialer: current,
			}
			port := route.Port
			if port == "" {
				port = "22"
			}
			if err := con.CreateClient(route.Addr, port, route.User, authMethods); err != nil {
				closeProxyConnectList(proxyConnects)
				return nil, nil, err
			}

			current = con.Client
			proxyConnects = append(proxyConnects, con)
		default:
			closeProxyConnectList(proxyConnects)
			return nil, nil, fmt.Errorf("sshlib: unsupported control persist proxy route type %q at index %d", route.Type, i)
		}
	}

	contextDialer, ok := current.(proxy.ContextDialer)
	if !ok {
		closeProxyConnectList(proxyConnects)
		return nil, nil, errors.New("sshlib: final control persist proxy route dialer does not implement proxy.ContextDialer")
	}

	return contextDialer, proxyConnects, nil
}

func serializeControlPersistProxyRoutes(routes []ProxyRoute) ([]controlPersistProxyRoute, error) {
	if len(routes) == 0 {
		return nil, nil
	}

	definitions := make([]controlPersistProxyRoute, 0, len(routes))
	for i, route := range routes {
		def := controlPersistProxyRoute{
			Type:     route.Type,
			Addr:     route.Addr,
			Port:     route.Port,
			User:     route.User,
			Password: route.Password,
			Command:  route.Command,
		}

		if route.Type == "ssh" {
			if route.Auth == nil {
				return nil, fmt.Errorf("sshlib: proxy route[%d] type ssh requires Auth", i)
			}

			auth, err := route.Auth.resolved()
			if err != nil {
				return nil, err
			}
			def.Auth = auth
		}

		definitions = append(definitions, def)
	}

	return definitions, nil
}

func (r ProxyRoute) authMethods(prompt PromptFunc) ([]ssh.AuthMethod, error) {
	if r.Type != "ssh" {
		return nil, nil
	}
	if r.Auth == nil {
		return nil, errors.New("sshlib: proxy route type ssh requires Auth")
	}

	resolved, err := r.Auth.resolved()
	if err != nil {
		return nil, err
	}

	return createControlPersistAuthMethodsWithPrompt(resolved, prompt)
}

func (r ProxyRoute) portOrDefault() string {
	if r.Port != "" {
		return r.Port
	}
	if r.Type == "ssh" {
		return "22"
	}
	return r.Port
}

func closeProxyConnectList(connects []*Connect) error {
	var firstErr error
	for i := len(connects) - 1; i >= 0; i-- {
		if err := connects[i].Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

type httpProxy struct {
	host     string
	haveAuth bool
	username string
	password string
	forward  proxy.Dialer
}

// Dial return net.Conn via http proxy.
func (s *httpProxy) Dial(network, addr string) (net.Conn, error) {
	c, err := s.forward.Dial("tcp", s.host)
	if err != nil {
		return nil, err
	}

	reqURL, err := url.Parse("http://" + addr)
	if err != nil {
		c.Close()
		return nil, err
	}
	reqURL.Scheme = ""

	req, err := http.NewRequest("CONNECT", reqURL.String(), nil)
	if err != nil {
		c.Close()
		return nil, err
	}
	req.Close = false
	if s.haveAuth {
		req.SetBasicAuth(s.username, s.password)
	}
	req.Header.Set("User-Agent", "Poweredby Golang")

	err = req.Write(c)
	if err != nil {
		c.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(c), req)
	if err != nil {
		resp.Body.Close()
		c.Close()
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		c.Close()
		err = fmt.Errorf("Connect server using proxy error, StatusCode [%d]", resp.StatusCode)
		return nil, err
	}

	return c, nil
}

// newHttpProxy
func newHttpProxy(uri *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	s := new(httpProxy)
	s.host = uri.Host
	s.forward = forward
	if uri.User != nil {
		s.haveAuth = true
		s.username = uri.User.Username()
		s.password, _ = uri.User.Password()
	}
	return s, nil
}
