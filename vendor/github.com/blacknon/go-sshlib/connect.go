// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/proxy"
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
	ProxyDialer proxy.Dialer

	// Connect timeout second.
	ConnectTimeout int

	// SendKeepAliveMax and SendKeepAliveInterval
	SendKeepAliveMax      int
	SendKeepAliveInterval int

	// Session use tty flag.
	TTY bool

	// Forward ssh agent flag.
	ForwardAgent bool

	// ssh-agent interface.
	// agent.Agent or agent.ExtendedAgent
	Agent AgentInterface

	// Forward x11 flag.
	ForwardX11 bool

	// shell terminal log flag
	logging bool

	// terminal log add timestamp flag
	logTimestamp bool

	// terminal log path
	logFile string
}

// CreateClient set c.Client.
func (c *Connect) CreateClient(host, port, user string, authMethods []ssh.AuthMethod) (err error) {
	uri := net.JoinHostPort(host, port)

	timeout := 20
	if c.ConnectTimeout > 0 {
		timeout = c.ConnectTimeout
	}

	// Create new ssh.ClientConfig{}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(timeout) * time.Second,
	}

	// check Dialer
	if c.ProxyDialer == nil {
		c.ProxyDialer = proxy.Direct
	}

	// Dial to host:port
	netConn, err := c.ProxyDialer.Dial("tcp", uri)
	if err != nil {
		return
	}

	// Create new ssh connect
	sshCon, channel, req, err := ssh.NewClientConn(netConn, uri, config)
	if err != nil {
		return
	}

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
	interval := 30
	if c.SendKeepAliveInterval > 0 {
		interval = c.SendKeepAliveInterval
	}

	// keep alive max (default 5)
	max := 5
	if c.SendKeepAliveMax > 0 {
		max = c.SendKeepAliveMax
	}

	// keep alive counter
	i := 0
	for {
		// Send keep alive packet
		_, err := session.SendRequest("keepalive", true, nil)
		// _, _, err := c.Client.SendRequest("keepalive", true, nil)
		if err == nil {
			i = 0
		} else {
			i += 1
		}

		// check counter
		if max <= i {
			session.Close()
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
//
func RequestTty(session *ssh.Session) (err error) {
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	// Get terminal window size
	fd := int(os.Stdin.Fd())
	width, hight, err := terminal.GetSize(fd)
	if err != nil {
		return
	}

	// TODO(blacknon): 環境変数から取得する方式だと、Windowsでうまく動作するか不明なので確認して対処する
	term := os.Getenv("TERM")
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
