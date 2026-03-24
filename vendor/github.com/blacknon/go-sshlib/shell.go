// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lunixbochs/vtclean"
	"golang.org/x/crypto/ssh"
	terminal "golang.org/x/term"
)

type controlExitError struct {
	status int
}

func (e *controlExitError) Error() string {
	return fmt.Sprintf("sshlib: remote command exited with status %d", e.status)
}

// Shell connect login shell over ssh.
func (c *Connect) Shell(session *ssh.Session) (err error) {
	if c.isControlClient() {
		return c.runControlSession(controlRequest{Type: controlRequestShell, Options: c.controlSessionOptions(true)})
	}

	if session == nil {
		session, err = c.CreateSession()
		if err != nil {
			return err
		}
		defer session.Close()
	}

	// Input terminal Make raw
	var fd int
	if c.PtyRelayTty != nil {
		fd = int(c.PtyRelayTty.Fd())
	} else {
		fd = int(os.Stdin.Fd())
	}

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

	// set tty
	if c.PtyRelayTty != nil {
		session.Stdin = c.PtyRelayTty
		session.Stdout = c.PtyRelayTty
		session.Stderr = c.PtyRelayTty
	}

	// Start shell
	err = session.Shell()
	if err != nil {
		return
	}

	// keep alive packet
	go c.SendKeepAlive(session)

	// if tty is set, get signal winch
	if c.PtyRelayTty != nil {
		go c.ChangeWinSize(session)
	}

	err = session.Wait()
	if err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			return nil
		}
		return
	}

	return
}

// Shell connect command shell over ssh.
// Used to start a shell with a specified command.
func (c *Connect) CmdShell(session *ssh.Session, command string) (err error) {
	if c.isControlClient() {
		req := controlRequest{
			Type:    controlRequestCmdShell,
			Command: command,
			Options: c.controlSessionOptions(true),
		}
		return c.runControlSession(req)
	}

	if session == nil {
		session, err = c.CreateSession()
		if err != nil {
			return err
		}
		defer session.Close()
	}

	// Input terminal Make raw
	var fd int
	if c.PtyRelayTty != nil {
		fd = int(c.PtyRelayTty.Fd())
	} else {
		fd = int(os.Stdin.Fd())
	}

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

	// set tty
	if c.PtyRelayTty != nil {
		session.Stdin = c.PtyRelayTty
		session.Stdout = c.PtyRelayTty
		session.Stderr = c.PtyRelayTty
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
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			return nil
		}
		return
	}

	return
}

func (c *Connect) ChangeWinSize(session *ssh.Session) {
	// Get terminal window size
	var fd int
	if c.PtyRelayTty != nil {
		fd = int(c.PtyRelayTty.Fd())
	} else {
		fd = int(os.Stdout.Fd())
	}
	width, height, err := terminal.GetSize(fd)
	if err != nil {
		return
	}

	// Send window size
	session.WindowChange(height, width)
}

// SetLog set up terminal log logging.
// This only happens in Connect.Shell().
func (c *Connect) SetLog(path string, timestamp bool) {
	c.logging = true
	c.logFile = path
	c.logTimestamp = timestamp
}

func (c *Connect) SetLogWithRemoveAnsiCode(path string, timestamp bool) {
	c.logging = true
	c.logFile = path
	c.logTimestamp = timestamp
	c.logRemoveAnsiCode = true
}

// logger is logging terminal log to c.logFile
func (c *Connect) logger(session *ssh.Session) (err error) {
	stdout, stderr, err := c.logWriters(session.Stdout, session.Stderr)
	if err != nil {
		return
	}

	session.Stdout = stdout
	session.Stderr = stderr
	return nil
}

func (c *Connect) logWriters(stdout, stderr io.Writer) (io.Writer, io.Writer, error) {
	logfile, err := os.OpenFile(c.logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, nil, err
	}

	if !c.logTimestamp && !c.logRemoveAnsiCode {
		return io.MultiWriter(stdout, logfile), io.MultiWriter(stderr, logfile), nil
	}

	buf := new(bytes.Buffer)
	logStdout := io.MultiWriter(stdout, buf)
	logStderr := io.MultiWriter(stderr, buf)

	go func() {
		preLine := []byte{}
		for {
			if buf.Len() > 0 {
				line, err := buf.ReadBytes('\n')

				if err == io.EOF {
					preLine = append(preLine, line...)
					continue
				}

				printLine := string(append(preLine, line...))

				if c.logTimestamp {
					timestamp := time.Now().Format("2006/01/02 15:04:05 ")
					printLine = timestamp + printLine
				}

				if c.logRemoveAnsiCode {
					printLine = "." + printLine
					printLine = vtclean.Clean(printLine, false)
				}

				fmt.Fprint(logfile, printLine)
				preLine = []byte{}
			} else {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	return logStdout, logStderr, nil
}

func (c *Connect) setupShell(session *ssh.Session) (err error) {
	// set FD
	stdin := GetStdin()
	session.Stdin = stdin
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

func (c *Connect) runControlSession(req controlRequest) error {
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

	input := GetStdin()
	output := io.Writer(os.Stdout)
	errput := io.Writer(os.Stderr)
	if c.PtyRelayTty != nil {
		input = c.PtyRelayTty
		output = c.PtyRelayTty
		errput = c.PtyRelayTty
	}

	if c.logging {
		output, errput, err = c.logWriters(output, errput)
		if err != nil {
			return err
		}
	}

	var fd int
	if c.PtyRelayTty != nil {
		fd = int(c.PtyRelayTty.Fd())
	} else {
		fd = int(os.Stdin.Fd())
	}

	state, err := terminal.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer terminal.Restore(fd, state)

	if req.Options.TTY {
		_ = c.sendControlWindowSize(writer)
		go c.watchControlWindowSize(writer)
	}

	go c.copyControlInput(writer, input)

	err = c.copyControlOutput(conn, output, errput)
	if req.Type == controlRequestShell {
		var exitErr *controlExitError
		if errors.As(err, &exitErr) {
			return nil
		}
	}
	return err
}

func (c *Connect) controlSessionOptions(forceTTY bool) controlSessionOptions {
	width := 80
	height := 24
	fd := int(os.Stdout.Fd())
	if c.PtyRelayTty != nil {
		fd = int(c.PtyRelayTty.Fd())
	}
	if w, h, err := terminal.GetSize(fd); err == nil {
		width = w
		height = h
	}

	return controlSessionOptions{
		TTY:               forceTTY,
		Term:              os.Getenv("TERM"),
		Width:             width,
		Height:            height,
		ForwardX11:        c.ForwardX11,
		ForwardX11Trusted: c.ForwardX11Trusted,
		ForwardAgent:      c.ForwardAgent,
	}
}

func (c *Connect) sendControlWindowSize(writer *lockedFrameWriter) error {
	fd := int(os.Stdout.Fd())
	if c.PtyRelayTty != nil {
		fd = int(c.PtyRelayTty.Fd())
	}

	width, height, err := terminal.GetSize(fd)
	if err != nil {
		return err
	}

	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[:4], uint32(width))
	binary.BigEndian.PutUint32(payload[4:], uint32(height))
	return writer.WriteFrame(streamFrameWindowChange, payload)
}

func (c *Connect) watchControlWindowSize(writer *lockedFrameWriter) {
	winch := syscall.Signal(0x1c)
	signalchan := make(chan os.Signal, 1)
	signal.Notify(signalchan, winch)
	for range signalchan {
		_ = c.sendControlWindowSize(writer)
	}
}

func (c *Connect) copyControlInput(writer *lockedFrameWriter, input io.Reader) {
	buf := make([]byte, 32*1024)
	for {
		n, err := input.Read(buf)
		if n > 0 {
			if writeErr := writer.WriteFrame(streamFrameStdin, buf[:n]); writeErr != nil {
				return
			}
		}
		if err != nil {
			_ = writer.WriteFrame(streamFrameCloseStdin, nil)
			return
		}
	}
}

func (c *Connect) copyControlOutput(conn net.Conn, stdout, stderr io.Writer) error {
	for {
		frameType, payload, err := readStreamFrame(conn)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		switch frameType {
		case streamFrameStdout:
			if _, err := stdout.Write(payload); err != nil {
				return err
			}
		case streamFrameStderr:
			if _, err := stderr.Write(payload); err != nil {
				return err
			}
		case streamFrameError:
			if len(payload) > 0 {
				_, _ = fmt.Fprintln(stderr, string(payload))
			}
		case streamFrameExit:
			if len(payload) != 4 {
				return nil
			}
			code := int(binary.BigEndian.Uint32(payload))
			if code == 0 {
				return nil
			}
			return &controlExitError{status: code}
		}
	}
}
