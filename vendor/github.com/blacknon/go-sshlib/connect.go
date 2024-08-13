// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
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

	// Session
	Session *ssh.Session

	// Session Stdin, Stdout, Stderr...
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// ProxyDialer
	ProxyDialer proxy.ContextDialer

	// Connect timeout second.
	ConnectTimeout int

	// SendKeepAliveMax and SendKeepAliveInterval
	SendKeepAliveMax      int
	SendKeepAliveInterval int

	// Session use tty flag.
	// Set it before CraeteClient.
	TTY bool

	// Forward ssh agent flag.
	// Set it before CraeteClient.
	ForwardAgent bool

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

	// Dynamic forward related logger
	DynamicForwardLogger *log.Logger

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
	uri := net.JoinHostPort(host, port)

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
			config.HostKeyCallback = c.verifyAndAppendNew
		} else {
			config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
		}
	}

	// check Dialer
	if c.ProxyDialer == nil {
		c.ProxyDialer = proxy.Direct
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.ConnectTimeout)*time.Second)
	defer cancel()

	// Dial to host:port
	netConn, cerr := c.ProxyDialer.DialContext(ctx, "tcp", uri)
	if cerr != nil {
		return cerr
	}

	// Set deadline
	netConn.SetDeadline(time.Now().Add(time.Duration(c.ConnectTimeout) * time.Second))

	// Create new ssh connect
	sshCon, channel, req, cerr := ssh.NewClientConn(netConn, uri, config)
	if cerr != nil {
		return cerr
	}

	// Reet deadline
	netConn.SetDeadline(time.Time{})

	// Create *ssh.Client
	c.Client = ssh.NewClient(sshCon, channel, req)

	return
}

// CreateSession retrun ssh.Session
func (c *Connect) CreateSession() (session *ssh.Session, err error) {
	// Create session
	session, err = c.Client.NewSession()
	return
}

// SendKeepAlive send packet to session.
// TODO(blacknon): Interval及びMaxを設定できるようにする(v0.1.1)
func (c *Connect) SendKeepAlive(session *ssh.Session) {
	// keep alive interval (default 30 sec)
	interval := 1
	if c.SendKeepAliveInterval > 0 {
		interval = c.SendKeepAliveInterval
	}

	for {
		// timeout channel
		tc := make(chan bool, 1)

		go func() {
			// Send keep alive packet
			_, err := session.SendRequest("keepalive", true, nil)
			if err == nil {
				tc <- true
			}
		}()

		select {
		case <-tc:
		case <-time.After(time.Duration(c.ConnectTimeout) * time.Second):
			session.Close()
			c.Client.Close()
			log.Println("keepalive timeout")
			return
		}

		// sleep
		time.Sleep(time.Duration(interval) * time.Second)
	}
}

// CheckClientAlive check alive ssh.Client.
func (c *Connect) CheckClientAlive() error {
	_, _, err := c.Client.SendRequest("keepalive", true, nil)
	if err == nil || err.Error() == "request failed" {
		return nil
	}
	return err
}

// RequestTty requests the association of a pty with the session on the remote
// host. Terminal size is obtained from the currently connected terminal
func RequestTty(session *ssh.Session) (err error) {
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	// Get terminal window size
	fd := int(os.Stdout.Fd())
	width, hight, err := terminal.GetSize(fd)
	if err != nil {
		return
	}

	// Get env `TERM`
	term := os.Getenv("TERM")
	if len(term) == 0 {
		term = "xterm"
	}

	if err = session.RequestPty(term, hight, width, modes); err != nil {
		session.Close()
		return
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
				width, hight, _ = terminal.GetSize(fd)
				session.WindowChange(hight, width)
			}
		}
	}()

	return
}
