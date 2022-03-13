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

	// setup options
	err = c.setOption(c.Session)
	if err != nil {
		return
	}

	// Set Stdin, Stdout, Stderr...
	if c.Stdin != nil {
		w, _ := c.Session.StdinPipe()
		go io.Copy(w, c.Stdin)
	} else {
		stdin := GetStdin()
		c.Session.Stdin = stdin
	}

	if c.Stdout != nil {
		or, _ := c.Session.StdoutPipe()
		go io.Copy(c.Stdout, or)
	} else {
		c.Session.Stdout = os.Stdout
	}

	if c.Stderr != nil {
		er, _ := c.Session.StderrPipe()
		go io.Copy(c.Stderr, er)
	} else {
		c.Session.Stderr = os.Stderr
	}

	// Run Command
	c.Session.Run(command)

	return
}

//
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
