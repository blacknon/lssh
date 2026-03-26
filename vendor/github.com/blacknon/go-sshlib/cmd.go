// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"io"
	"log"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
)

// Command connect and run command over ssh.
// Output data is processed by channel because it is executed in parallel. If specification is troublesome, it is good to generate and process session from ssh package.
func (c *Connect) Command(command string) (err error) {
	if c.isControlClient() {
		req := controlRequest{
			Type:    controlRequestCommand,
			Command: command,
			Options: c.controlSessionOptions(c.TTY),
		}
		return c.runControlCommand(req)
	}

	// create session
	if c.Session == nil {
		c.Session, err = c.CreateSession()
		if err != nil {
			return
		}
	}
	defer func() { c.Session = nil }()

	// Set Stdin
	switch {
	case c.Stdin != nil:
		w, _ := c.Session.StdinPipe()
		go io.Copy(w, c.Stdin)

	case c.PtyRelayTty != nil:
		c.Session.Stdin = c.PtyRelayTty

	default:
		stdin := GetStdin()
		c.Session.Stdin = stdin
	}

	// Set Stdout
	switch {
	case c.Stdout != nil:
		or, _ := c.Session.StdoutPipe()
		go io.Copy(c.Stdout, or)

	case c.PtyRelayTty != nil:
		c.Session.Stdout = c.PtyRelayTty

	default:
		c.Session.Stdout = os.Stdout
	}

	// Set Stderr
	switch {
	case c.Stderr != nil:
		er, _ := c.Session.StderrPipe()
		go io.Copy(c.Stderr, er)

	case c.PtyRelayTty != nil:
		c.Session.Stderr = c.PtyRelayTty

	default:
		c.Session.Stderr = os.Stderr
	}

	// setup options
	err = c.setOption(c.Session)
	if err != nil {
		return
	}

	err = c.Session.Start(command)
	if err != nil {
		return
	}

	stopKeepAlive := c.startSessionKeepAlive(c.Session)
	defer stopKeepAlive()

	err = c.Session.Wait()

	return
}

func (c *Connect) runControlCommand(req controlRequest) error {
	resp, err := c.requestControl(req)
	if err != nil {
		return err
	}

	conn, err := net.Dial("unix", resp.StreamPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	writer := &lockedFrameWriter{w: conn}

	stdin := io.Reader(GetStdin())
	if c.Stdin != nil {
		stdin = c.Stdin
	} else if c.PtyRelayTty != nil {
		stdin = c.PtyRelayTty
	}

	stdout := io.Writer(os.Stdout)
	if c.Stdout != nil {
		stdout = c.Stdout
	} else if c.PtyRelayTty != nil {
		stdout = c.PtyRelayTty
	}

	stderr := io.Writer(os.Stderr)
	if c.Stderr != nil {
		stderr = c.Stderr
	} else if c.PtyRelayTty != nil {
		stderr = c.PtyRelayTty
	}

	if c.logging {
		stdout, stderr, err = c.logWriters(stdout, stderr)
		if err != nil {
			return err
		}
	}

	go c.copyControlInput(writer, stdin)

	if req.Options.TTY {
		_ = c.sendControlWindowSize(writer)
		go c.watchControlWindowSize(writer)
	}

	return c.copyControlOutput(conn, stdout, stderr)
}

func (c *Connect) setOption(session *ssh.Session) (err error) {
	// Request tty
	if c.TTY {
		err = RequestTty(session)
		if err != nil {
			return err
		}
	}

	// ssh agent forwarding
	if c.ForwardAgent {
		c.ForwardSshAgent(session)
	}

	// x11 forwarding
	if c.ForwardX11 {
		err = c.X11Forward(session)
		if err != nil {
			log.Println(err)
		}
		err = nil
	}

	return
}
