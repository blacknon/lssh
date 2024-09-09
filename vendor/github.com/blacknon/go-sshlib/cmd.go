// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"io"
	"log"
	"os"

	"golang.org/x/crypto/ssh"
)

// Command connect and run command over ssh.
// Output data is processed by channel because it is executed in parallel. If specification is troublesome, it is good to generate and process session from ssh package.
func (c *Connect) Command(command string) (err error) {
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

	// Run Command
	c.Session.Run(command)

	return
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
