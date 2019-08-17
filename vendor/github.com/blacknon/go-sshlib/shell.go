// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

// Shell connect login shell over ssh.
func (c *Connect) Shell(session *ssh.Session) (err error) {
	// Input terminal Make raw
	fd := int(os.Stdin.Fd())
	state, err := terminal.MakeRaw(fd)
	if err != nil {
		return
	}
	defer terminal.Restore(fd, state)

	// setup
	err = c.setupShell(session)
	if err != nil {
		return
	}

	// Start shell
	err = session.Shell()
	if err != nil {
		return
	}

	// keep alive packet
	go c.SendKeepAlive(session)

	err = session.Wait()
	if err != nil {
		return
	}

	return
}

// Shell connect command shell over ssh.
// Used to start a shell with a specified command.
func (c *Connect) CmdShell(session *ssh.Session, command string) (err error) {
	// Input terminal Make raw
	fd := int(os.Stdin.Fd())
	state, err := terminal.MakeRaw(fd)
	if err != nil {
		return
	}
	defer terminal.Restore(fd, state)

	// setup
	err = c.setupShell(session)
	if err != nil {
		return
	}

	// Start shell
	err = session.Start(command)
	if err != nil {
		return
	}

	// keep alive packet
	go c.SendKeepAlive(session)

	err = session.Wait()
	if err != nil {
		return
	}

	return
}

func (c *Connect) setupShell(session *ssh.Session) (err error) {
	// set FD
	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	// Logging
	if c.logging {
		err = c.logger(session)
		if err != nil {
			log.Println(err)
		}
	}
	err = nil

	// Request tty
	err = RequestTty(session)
	if err != nil {
		return err
	}

	// x11 forwarding
	if c.ForwardX11 {
		err = c.X11Forward(session)
		if err != nil {
			log.Println(err)
		}
	}
	err = nil

	// ssh agent forwarding
	if c.ForwardAgent {
		c.ForwardSshAgent(session)
	}

	return
}

// SetLog set up terminal log logging.
// This only happens in Connect.Shell().
func (c *Connect) SetLog(path string, timestamp bool) {
	c.logging = true
	c.logFile = path
	c.logTimestamp = timestamp
}

// logger is logging terminal log to c.logFile
// TODO(blacknon): Writerを利用した処理方法に変更する(v0.1.1)
func (c *Connect) logger(session *ssh.Session) (err error) {
	logfile, err := os.OpenFile(c.logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return
	}

	if c.logTimestamp {
		buf := new(bytes.Buffer)
		session.Stdout = io.MultiWriter(session.Stdout, buf)
		session.Stderr = io.MultiWriter(session.Stderr, buf)

		go func() {
			preLine := []byte{}
			for {
				if buf.Len() > 0 {
					line, err := buf.ReadBytes('\n')

					if err == io.EOF {
						preLine = append(preLine, line...)
						continue
					} else {
						timestamp := time.Now().Format("2006/01/02 15:04:05 ") // yyyy/mm/dd HH:MM:SS
						fmt.Fprintf(logfile, timestamp+string(append(preLine, line...)))
						preLine = []byte{}
					}
				} else {
					time.Sleep(10 * time.Millisecond)
				}
			}
		}()

	} else {
		session.Stdout = io.MultiWriter(session.Stdout, logfile)
		session.Stderr = io.MultiWriter(session.Stderr, logfile)
	}

	return err
}
