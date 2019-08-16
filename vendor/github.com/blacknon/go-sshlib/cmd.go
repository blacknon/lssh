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

// Cmd connect and run command over ssh.
// Output data is processed by channel because it is executed in parallel. If specification is troublesome, it is good to generate and process session from ssh package.
// TODO(blacknon): writer/readerによる入出力に書き換える(stdinは特に)。 (対応: v0.1.1)。
func (c *Connect) Cmd(command string, output chan []byte) (err error) {
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
	if len(c.Stdin) > 0 {
		c.session.Stdin = bytes.NewReader(c.Stdin)
	} else {
		c.session.Stdin = os.Stdin
	}

	// Set output buffer
	buf := new(bytes.Buffer)

	// set output
	// TODO(blacknon): bufferは可能な限り使わず、Readerを渡すようにしたい
	c.session.Stdout = io.MultiWriter(buf)
	c.session.Stderr = io.MultiWriter(buf)
	if c.ForceStd {
		// Input terminal Make raw
		fd := int(os.Stdin.Fd())
		state, _ := terminal.MakeRaw(fd)
		defer terminal.Restore(fd, state)

		c.session.Stdout = os.Stdout
		c.session.Stderr = os.Stderr
	}

	// Run Command
	isExit := make(chan bool)
	go func() {
		c.session.Run(command)
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

// sendCmdOutput send to output channel.
// TODO(blacknon): Writer/Readerでの入出力に切り替えたら削除。(対応: v0.1.1)
func sendCmdOutput(buf *bytes.Buffer, output chan []byte, isExit <-chan bool) {
	// TODO(blacknon): bufferは使わず、Readerを渡すようにしたい
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
