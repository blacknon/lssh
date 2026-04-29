// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
	terminal "golang.org/x/term"
)

// Connect structure to store contents about ssh connection.
type Connect struct {
	// Client *ssh.Client
	Client *ssh.Client

	reconnectMu    sync.Mutex
	reconnectAuths []ssh.AuthMethod
	controlClient  *controlClient
	controlMaster  *controlMaster
	controlHost    string
	controlPort    string
	controlUser    string
	controlSpawned bool
	proxyConnects  []*Connect

	// Session
	Session *ssh.Session

	// Session Stdin, Stdout, Stderr...
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// ProxyDialer
	ProxyDialer proxy.ContextDialer

	// ProxyRoute takes precedence over ProxyDialer when set.
	ProxyRoute []ProxyRoute

	// Connect timeout second.
	ConnectTimeout int

	// AutoReconnect reconnects the underlying SSH transport before starting a new operation
	// when the direct connection has been lost.
	AutoReconnect bool

	// AutoReconnectInterval is the delay in seconds between reconnect attempts.
	AutoReconnectInterval int

	// AutoReconnectMax is the maximum number of reconnect attempts.
	AutoReconnectMax int

	// SendKeepAliveMax and SendKeepAliveInterval
	SendKeepAliveMax      int
	SendKeepAliveInterval int

	// Session use tty flag.
	// Set it before CraeteClient.
	TTY bool

	// Forward ssh agent flag.
	// Set it before CraeteClient.
	ForwardAgent bool

	// Set the TTY to be used as the input and output for the Session/Cmd.
	PtyRelayTty *os.File

	// StdoutMutex is a mutex for use Stdout.
	StdoutMutex *sync.Mutex

	// CheckKnownHosts if true, check knownhosts.
	// Ignored if HostKeyCallback is set.
	// Set it before CraeteClient.
	CheckKnownHosts bool

	// HostKeyCallback is ssh.HostKeyCallback.
	// This item takes precedence over `CheckKnownHosts`.
	// Set it before CraeteClient.
	HostKeyCallback ssh.HostKeyCallback

	// OverwriteKnownHosts if true, if the knownhost is different, check whether to overwrite.
	OverwriteKnownHosts bool

	// KnownHostsFiles is list of knownhosts files path.
	KnownHostsFiles []string

	// TextAskWriteKnownHosts defines a confirmation message when writing a knownhost.
	// We are using Go's template engine and have the following variables available.
	// - Address ... ssh server hostname
	// - RemoteAddr ... ssh server address
	// - Fingerprint ... ssh PublicKey fingerprint
	TextAskWriteKnownHosts string

	// TextAskOverwriteKnownHosts defines a confirmation message when over-writing a knownhost.
	// We are using Go's template engine and have the following variables available.
	// - Address ... ssh server hostname
	// - RemoteAddr ... ssh server address
	// - OldKeyText ... old ssh PublicKey text.
	//                  ex: /home/user/.ssh/known_hosts:17: ecdsa-sha2-nistp256 AAAAE2VjZHN...bJklasnFtkFSDyOjTFSv2g=
	// - NewFingerprint ... new ssh PublicKey fingerprint
	TextAskOverwriteKnownHosts string

	// ssh-agent interface.
	// agent.Agent or agent.ExtendedAgent
	// Set it before CraeteClient.
	Agent AgentInterface

	// Forward x11 flag.
	// Set it before CraeteClient.
	ForwardX11 bool

	// Forward X11 trusted flag.
	// This flag is ssh -Y option like flag.
	// Set it before CraeteClient.
	ForwardX11Trusted bool

	x11HandlerOnce sync.Once

	// Dynamic forward related logger
	DynamicForwardLogger *log.Logger

	// ControlMaster enables OpenSSH-like connection sharing via a local control socket.
	// Supported values are "", "no", "yes", and "auto".
	ControlMaster string

	// ControlPath is the Unix domain socket path used for connection sharing.
	ControlPath string

	// ControlPersist keeps the local control socket alive while the owning process remains alive.
	// When greater than zero, sshlib will try to start a detached helper process.
	ControlPersist time.Duration

	// ControlPersistAuth contains auth methods that can be replayed by the detached helper.
	// Set ControlPersistAuth.AuthMethods with auth methods created by
	// sshlib.CreateAuthMethodPassword or sshlib.CreateAuthMethodPublicKey.
	// Required when ControlPersist > 0 and no master is already running.
	ControlPersistAuth *ControlPersistAuth

	// shell terminal log flag
	logging bool

	// terminal log add timestamp flag
	logTimestamp bool

	// terminal log path
	logFile string

	// remove ansi code on terminal log.
	logRemoveAnsiCode bool
}

// CreateClient set c.Client.
func (c *Connect) CreateClient(host, port, user string, authMethods []ssh.AuthMethod) (err error) {
	debugf("sshlib: CreateClient host=%s port=%s user=%s control_master=%s persist=%s proxy_route=%d proxy_dialer=%t\n",
		host, port, user, c.ControlMaster, c.ControlPersist, len(c.ProxyRoute), c.ProxyDialer != nil)
	c.controlClient = nil
	c.controlSpawned = false
	c.controlHost = host
	c.controlPort = port
	c.controlUser = user

	authMethods, err = c.resolveAuthMethods(authMethods, nil)
	if err != nil {
		return err
	}
	c.rememberReconnectConfig(host, port, user, authMethods)

	mode := c.controlMode()
	if mode == "" || mode == "no" {
		debugln("sshlib: CreateClient using direct mode")
		return c.createDirectClient(host, port, user, authMethods)
	}

	if c.ControlPath == "" {
		return errors.New("sshlib: ControlPath is required when ControlMaster is enabled")
	}

	if mode == "auto" || mode == "yes" {
		debugf("sshlib: attempting existing control socket path=%s\n", c.ControlPath)
		client, cerr := dialControlClient(c.ControlPath)
		if cerr == nil {
			debugln("sshlib: connected to existing control master")
			c.controlClient = client
			return nil
		}
		debugf("sshlib: no existing control master path=%s err=%v\n", c.ControlPath, cerr)

		if mode == "yes" {
			return cerr
		}
	}

	if c.ControlPersist > 0 {
		debugln("sshlib: spawning detached control master")
		if err := c.startDetachedControlMaster(host, port, user); err != nil {
			return err
		}
		c.controlSpawned = true

		client, err := waitForControlClient(c.ControlPath, 5*time.Second)
		if err != nil {
			debugf("sshlib: waiting for control client failed path=%s err=%v\n", c.ControlPath, err)
			return err
		}
		debugln("sshlib: detached control master ready")
		c.controlClient = client
		return nil
	}

	if err := c.createDirectClient(host, port, user, authMethods); err != nil {
		return err
	}

	master, err := newControlMaster(c, c.ControlPath)
	if err != nil {
		c.Client.Close()
		c.Client = nil
		return err
	}
	c.controlMaster = master

	return nil
}

func (c *Connect) resolveAuthMethods(authMethods []ssh.AuthMethod, prompt PromptFunc) ([]ssh.AuthMethod, error) {
	if len(authMethods) > 0 {
		return authMethods, nil
	}
	if c.ControlPersistAuth == nil {
		return authMethods, nil
	}

	resolved, err := c.ControlPersistAuth.resolved()
	if err != nil {
		return nil, err
	}

	return createControlPersistAuthMethodsWithPrompt(resolved, prompt)
}

func (c *Connect) IsControlClient() bool {
	return c.isControlClient()
}

func (c *Connect) SpawnedControlMaster() bool {
	return c.controlSpawned
}

func (c *Connect) createDirectClient(host, port, user string, authMethods []ssh.AuthMethod) (err error) {
	uri := net.JoinHostPort(host, port)
	debugf("sshlib: createDirectClient begin uri=%s user=%s timeout=%ds\n", uri, user, c.ConnectTimeout)

	timeout := 20
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = timeout
	}

	// Create new ssh.ClientConfig{}
	config := &ssh.ClientConfig{
		User:    user,
		Auth:    authMethods,
		Timeout: time.Duration(c.ConnectTimeout) * time.Second,
	}

	if c.HostKeyCallback != nil {
		config.HostKeyCallback = c.HostKeyCallback
	} else {
		if c.CheckKnownHosts {
			if len(c.KnownHostsFiles) == 0 {
				// append default files
				c.KnownHostsFiles = append(c.KnownHostsFiles, "~/.ssh/known_hosts")
			}
			config.HostKeyCallback = c.VerifyAndAppendNew
		} else {
			config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
		}
	}

	// check Dialer
	dialer := c.ProxyDialer
	proxyConnects := c.proxyConnects
	if len(c.ProxyRoute) > 0 {
		if err := c.closeProxyConnects(); err != nil {
			return err
		}
		dialer, proxyConnects, err = buildProxyRouteDialer(c.ProxyRoute, nil)
		if err != nil {
			return err
		}
	} else if dialer == nil {
		dialer = proxy.Direct
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ConnectTimeout)*time.Second)
	defer cancel()

	// Dial to host:port
	debugf("sshlib: dialing network=tcp addr=%s\n", uri)
	netConn, cerr := dialer.DialContext(ctx, "tcp", uri)
	if cerr != nil {
		debugf("sshlib: dial failed addr=%s err=%v\n", uri, cerr)
		_ = closeProxyConnectList(proxyConnects)
		return cerr
	}
	debugf("sshlib: dial succeeded addr=%s\n", uri)

	// Set deadline
	_ = netConn.SetDeadline(time.Now().Add(time.Duration(c.ConnectTimeout) * time.Second))

	// Create new ssh connect
	debugf("sshlib: starting ssh handshake addr=%s\n", uri)
	sshCon, channel, req, cerr := ssh.NewClientConn(netConn, uri, config)
	if cerr != nil {
		debugf("sshlib: ssh handshake failed addr=%s err=%v\n", uri, cerr)
		_ = netConn.Close()
		_ = closeProxyConnectList(proxyConnects)
		return cerr
	}
	debugf("sshlib: ssh handshake succeeded addr=%s\n", uri)

	// Reet deadline
	_ = netConn.SetDeadline(time.Time{})

	// Create *ssh.Client
	c.Client = ssh.NewClient(sshCon, channel, req)
	c.proxyConnects = proxyConnects
	debugf("sshlib: createDirectClient success uri=%s\n", uri)

	return
}

func (c *Connect) rememberReconnectConfig(host, port, user string, authMethods []ssh.AuthMethod) {
	c.controlHost = host
	c.controlPort = port
	c.controlUser = user
	c.reconnectAuths = append(c.reconnectAuths[:0], authMethods...)
}

func (c *Connect) controlMode() string {
	switch c.ControlMaster {
	case "", "no":
		return c.ControlMaster
	case "yes", "auto":
		return c.ControlMaster
	default:
		return ""
	}
}

func (c *Connect) isControlClient() bool {
	return c.controlClient != nil
}

// Close releases control resources and the underlying SSH client.
func (c *Connect) Close() error {
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	var err error

	if c.controlClient != nil {
		err = c.controlClient.Close()
		c.controlClient = nil
	}

	if c.controlMaster != nil {
		closeErr := c.controlMaster.Close()
		c.controlMaster = nil
		if err == nil {
			err = closeErr
		}
	}

	if c.Client != nil {
		closeErr := c.Client.Close()
		c.Client = nil
		if err == nil {
			err = closeErr
		}
	}

	closeErr := c.closeProxyConnects()
	if err == nil {
		err = closeErr
	}

	return err
}

func (c *Connect) closeProxyConnects() error {
	err := closeProxyConnectList(c.proxyConnects)
	c.proxyConnects = nil
	return err
}

// CreateSession retrun ssh.Session
func (c *Connect) CreateSession() (session *ssh.Session, err error) {
	if c.isControlClient() {
		return nil, errors.New("sshlib: CreateSession is not available over ControlMaster; use Command, Shell(nil), or CmdShell(nil, command)")
	}

	if err := c.ensureActiveConnection(); err != nil {
		return nil, err
	}

	// Create session
	session, err = c.Client.NewSession()
	return
}

// Dial opens a connection using the active SSH transport.
// When ControlMaster is enabled, the dial is tunneled via the control socket.
func (c *Connect) Dial(network, addr string) (net.Conn, error) {
	if c.isControlClient() {
		if err := c.ensureActiveConnection(); err != nil {
			return nil, err
		}
		return c.controlClient.Dial(network, addr)
	}
	if err := c.ensureActiveConnection(); err != nil {
		return nil, err
	}
	return c.Client.Dial(network, addr)
}

// Listen starts a remote listener using the active SSH transport.
// When ControlMaster is enabled, the listener is managed by the control master.
func (c *Connect) Listen(network, addr string) (net.Listener, error) {
	if c.isControlClient() {
		if err := c.ensureActiveConnection(); err != nil {
			return nil, err
		}
		return c.controlClient.Listen(network, addr)
	}
	if err := c.ensureActiveConnection(); err != nil {
		return nil, err
	}
	return c.Client.Listen(network, addr)
}

func (c *Connect) autoReconnectConfig() (time.Duration, int) {
	interval := 1
	if c.AutoReconnectInterval > 0 {
		interval = c.AutoReconnectInterval
	}

	max := 1
	if c.AutoReconnectMax > 0 {
		max = c.AutoReconnectMax
	}

	return time.Duration(interval) * time.Second, max
}

func (c *Connect) canAutoReconnect() bool {
	return c.AutoReconnect && !c.isControlClient() && c.controlHost != "" && c.controlPort != "" && c.controlUser != "" && len(c.reconnectAuths) > 0
}

func (c *Connect) ensureActiveConnection() error {
	if c.isControlClient() {
		return c.ensureControlClient()
	}

	if c.Client == nil {
		if !c.canAutoReconnect() {
			return errors.New("ssh client is nil")
		}
		return c.reconnect()
	}

	if err := c.CheckClientAlive(); err != nil {
		if !c.canAutoReconnect() {
			return err
		}
		return c.reconnect()
	}

	return nil
}

func (c *Connect) reconnect() error {
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	if c.Client != nil {
		if err := c.CheckClientAlive(); err == nil {
			return nil
		}
	}

	if !c.canAutoReconnect() {
		if c.Client == nil {
			return errors.New("ssh client is nil")
		}
		return errors.New("sshlib: auto reconnect is not configured")
	}

	interval, max := c.autoReconnectConfig()
	var lastErr error

	for attempt := 0; attempt < max; attempt++ {
		if attempt > 0 {
			time.Sleep(interval)
		}

		if c.Client != nil {
			_ = c.Client.Close()
			c.Client = nil
		}
		_ = c.closeProxyConnects()

		lastErr = c.createDirectClient(c.controlHost, c.controlPort, c.controlUser, c.reconnectAuths)
		if lastErr == nil {
			return nil
		}
	}

	return lastErr
}

func (c *Connect) keepAliveConfig() (time.Duration, int) {
	interval := 30
	if c.SendKeepAliveInterval > 0 {
		interval = c.SendKeepAliveInterval
	}

	max := 3
	if c.SendKeepAliveMax > 0 {
		max = c.SendKeepAliveMax
	}

	return time.Duration(interval) * time.Second, max
}

func (c *Connect) startSessionKeepAlive(session *ssh.Session) func() {
	done := make(chan struct{})

	go func() {
		interval, max := c.keepAliveConfig()
		t := time.NewTicker(interval)
		defer t.Stop()

		failures := 0
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if _, err := session.SendRequest("keepalive@openssh.com", true, nil); err != nil {
					log.Println("Failed to send keepalive packet:", err)
					failures++
					if failures > max {
						_ = session.Close()
						return
					}
					continue
				}

				failures = 0
			}
		}
	}()

	return func() {
		close(done)
	}
}

// SendKeepAlive send packet to session.
// TODO(blacknon): Interval及びMaxを設定できるようにする(v0.1.1)
func (c *Connect) SendKeepAlive(session *ssh.Session) {
	interval, max := c.keepAliveConfig()
	t := time.NewTicker(interval)
	defer t.Stop()

	failures := 0
	for range t.C {
		if _, err := session.SendRequest("keepalive@openssh.com", true, nil); err != nil {
			log.Println("Failed to send keepalive packet:", err)
			failures++
			if failures > max {
				_ = session.Close()
				return
			}
			continue
		}

		failures = 0
	}
}

// CheckClientAlive check alive ssh.Client.
func (c *Connect) CheckClientAlive() error {
	if c.isControlClient() {
		return c.controlClient.Ping()
	}
	if c.Client == nil {
		return errors.New("ssh client is nil")
	}

	_, _, err := c.Client.SendRequest("keepalive@openssh.com", true, nil)
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "request failed") {
		return nil
	}
	return err
}

// RequestTty requests the association of a pty with the session on the remote
// host. Terminal size is obtained from the currently connected terminal
func RequestTty(session *ssh.Session) (err error) {
	// Get terminal window size
	fd := int(os.Stdout.Fd())
	width, height, err := terminal.GetSize(fd)
	if err != nil {
		return
	}

	// Get env `TERM`
	term := os.Getenv("TERM")
	if err = RequestTtyWithSize(session, term, width, height, nil); err != nil {
		return err
	}

	// Terminal resize goroutine.
	winch := syscall.Signal(0x1c)
	signalchan := make(chan os.Signal, 1)
	signal.Notify(signalchan, winch)
	go func() {
		for {
			s := <-signalchan
			switch s {
			case winch:
				fd := int(os.Stdout.Fd())
				width, height, _ = terminal.GetSize(fd)
				session.WindowChange(height, width)
			}
		}
	}()

	return
}

func defaultTerminalModes() ssh.TerminalModes {
	return ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
}

func normalizeTerminalTerm(term string) string {
	if term == "" {
		term = os.Getenv("TERM")
	}
	if term == "" {
		return "xterm-256color"
	}
	return term
}

func normalizeTerminalModes(modes ssh.TerminalModes) ssh.TerminalModes {
	if len(modes) == 0 {
		return defaultTerminalModes()
	}
	return modes
}

// RequestTtyWithSize requests the association of a pty with the session on the
// remote host using the caller-provided terminal size.
func RequestTtyWithSize(session *ssh.Session, term string, cols, rows int, modes ssh.TerminalModes) error {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	if err := session.RequestPty(normalizeTerminalTerm(term), rows, cols, normalizeTerminalModes(modes)); err != nil {
		session.Close()
		return err
	}

	return nil
}
