// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"bytes"
	"io"
	"log"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

// CmdWriter connect and run command over ssh.
// In order to be able to send in parallel from io.MultiWriter, it is made to receive Writer by channel.
func (c *Connect) CmdWriter(command string, output chan []byte, input chan io.Writer) (err error) {
	// create session
	if c.session == nil {
		c.session, err = c.CreateSession()
		if err != nil {
			close(output)
			return
		}
	}

	// setup
	err = c.setupCmd(c.session)
	if err != nil {
		return
	}

	// if set Stdin,
	writer, _ := c.session.StdinPipe()
	input <- writer
	defer writer.Close()

	// Set output buffer
	buf := new(bytes.Buffer)
	c.session.Stdout = io.MultiWriter(buf)
	c.session.Stderr = io.MultiWriter(buf)

	// make exit channel
	isExit := make(chan bool)

	// Run command
	c.session.Start(command)

	// Send output channel
	go sendCmdOutput(buf, output, isExit)

	// Run command wait
	c.session.Wait()
	isExit <- true

	c.session = nil

	return
}

// Cmd connect and run command over ssh.
// Output data is processed by channel because it is executed in parallel. If specification is troublesome, it is good to generate and process session from ssh package.
func (c *Connect) Cmd(command string, output chan []byte) (err error) {
	// create session
	session, err := c.CreateSession()
	if err != nil {
		close(output)
		return
	}

	// setup
	err = c.setupCmd(session)
	if err != nil {
		return
	}

	// if set Stdin,
	if len(c.Stdin) > 0 {
		session.Stdin = bytes.NewReader(c.Stdin)
	} else {
		session.Stdin = os.Stdin
	}

	// Set output buffer
	buf := new(bytes.Buffer)

	// set output
	session.Stdout = io.MultiWriter(buf)
	session.Stderr = io.MultiWriter(buf)
	if c.ForceStd {
		// Input terminal Make raw
		fd := int(os.Stdin.Fd())
		state, _ := terminal.MakeRaw(fd)
		defer terminal.Restore(fd, state)

		session.Stdout = os.Stdout
		session.Stderr = os.Stderr
	}

	// Run Command
	isExit := make(chan bool)
	go func() {
		session.Run(command)
		isExit <- true
	}()

	// Send output channel
	sendCmdOutput(buf, output, isExit)

	return
}

//
func (c *Connect) setupCmd(session *ssh.Session) (err error) {
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

// Kill c.session close.
// Only work c.CmdWriter().
func (c *Connect) Kill() {
	c.session.Signal(ssh.SIGINT)
	c.session.Close()
}

// sendCmdOutput send to output channel.
func sendCmdOutput(buf *bytes.Buffer, output chan []byte, isExit <-chan bool) {
	exit := false

GetOutputLoop:
	for {
		if buf.Len() > 0 {
			for {
				line, err := buf.ReadBytes('\n')
				if len(line) > 0 {
					output <- line
				}

				// if err is io.EOF
				if err == io.EOF {
					break
				}
			}
		} else {
			select {
			case <-isExit:
				exit = true
			case <-time.After(10 * time.Millisecond):
				if exit {
					break GetOutputLoop
				} else {
					continue GetOutputLoop
				}
			}
		}
	}

	close(output)
}
