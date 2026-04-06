// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
)

// Terminal is a low-level interactive terminal session.
type Terminal struct {
	Session *ssh.Session
	Stdin   io.WriteCloser
	Stdout  io.Reader
	Stderr  io.Reader

	conn      net.Conn
	writer    *lockedFrameWriter
	waitCh    chan error
	closeOnce sync.Once
}

// TerminalOptions configures OpenTerminal.
type TerminalOptions struct {
	Term       string
	Cols       int
	Rows       int
	Modes      ssh.TerminalModes
	StartShell bool
	Command    string
}

// OpenTerminal opens an interactive PTY session without forcing raw mode or
// wiring stdio relay.
func (c *Connect) OpenTerminal(opts TerminalOptions) (*Terminal, error) {
	if opts.StartShell && opts.Command != "" {
		return nil, errors.New("sshlib: StartShell and Command cannot both be set")
	}

	if c.isControlClient() {
		return c.openControlTerminal(opts)
	}

	session, err := c.CreateSession()
	if err != nil {
		return nil, err
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return nil, err
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, err
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		session.Close()
		return nil, err
	}

	if err := RequestTtyWithSize(session, opts.Term, opts.Cols, opts.Rows, opts.Modes); err != nil {
		return nil, err
	}

	if c.ForwardX11 {
		if err := c.X11Forward(session); err != nil {
			session.Close()
			return nil, err
		}
	}

	if c.ForwardAgent {
		c.ForwardSshAgent(session)
	}

	switch {
	case opts.StartShell:
		err = session.Shell()
	case opts.Command != "":
		err = session.Start(opts.Command)
	}
	if err != nil {
		session.Close()
		return nil, err
	}

	return &Terminal{
		Session: session,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
	}, nil
}

func (c *Connect) openControlTerminal(opts TerminalOptions) (*Terminal, error) {
	req := controlRequest{
		Options: c.controlSessionOptions(true),
	}
	req.Options.Term = opts.Term
	req.Options.Width = opts.Cols
	req.Options.Height = opts.Rows

	switch {
	case opts.StartShell:
		req.Type = controlRequestShell
	case opts.Command != "":
		req.Type = controlRequestCmdShell
		req.Command = opts.Command
	default:
		return nil, errors.New("sshlib: terminal requires shell or command")
	}

	resp, err := c.requestControl(req)
	if err != nil {
		return nil, err
	}

	conn, err := net.Dial("unix", resp.StreamPath)
	if err != nil {
		return nil, err
	}

	writer := &lockedFrameWriter{w: conn}
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	t := &Terminal{
		Stdout: stdoutReader,
		Stderr: stderrReader,
		conn:   conn,
		writer: writer,
		waitCh: make(chan error, 1),
	}
	t.Stdin = &controlTerminalStdin{writer: writer}

	go t.copyControlOutput(stdoutWriter, stderrWriter)

	return t, nil
}

type controlTerminalStdin struct {
	writer *lockedFrameWriter
}

func (w *controlTerminalStdin) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if w.writer == nil {
		return 0, io.ErrClosedPipe
	}
	if err := w.writer.WriteFrame(streamFrameStdin, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *controlTerminalStdin) Close() error {
	if w.writer == nil {
		return nil
	}
	return w.writer.WriteFrame(streamFrameCloseStdin, nil)
}

func (t *Terminal) copyControlOutput(stdoutWriter, stderrWriter *io.PipeWriter) {
	defer stdoutWriter.Close()
	defer stderrWriter.Close()

	for {
		frameType, payload, err := readStreamFrame(t.conn)
		if err != nil {
			if errors.Is(err, io.EOF) {
				t.waitCh <- nil
			} else {
				t.waitCh <- err
			}
			return
		}

		switch frameType {
		case streamFrameStdout:
			if _, err := stdoutWriter.Write(payload); err != nil {
				t.waitCh <- err
				return
			}
		case streamFrameStderr:
			if _, err := stderrWriter.Write(payload); err != nil {
				t.waitCh <- err
				return
			}
		case streamFrameError:
			if len(payload) > 0 {
				if _, err := stderrWriter.Write([]byte(fmt.Sprintln(string(payload)))); err != nil {
					t.waitCh <- err
					return
				}
			}
		case streamFrameExit:
			if len(payload) != 4 {
				t.waitCh <- nil
				return
			}
			code := int(binary.BigEndian.Uint32(payload))
			if code == 0 {
				t.waitCh <- nil
			} else {
				t.waitCh <- &controlExitError{status: code}
			}
			return
		}
	}
}

// Resize updates the remote PTY size.
func (t *Terminal) Resize(cols, rows int) error {
	if t == nil {
		return errors.New("sshlib: terminal session is nil")
	}
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	if t.writer != nil {
		payload := make([]byte, 8)
		binary.BigEndian.PutUint32(payload[:4], uint32(cols))
		binary.BigEndian.PutUint32(payload[4:], uint32(rows))
		return t.writer.WriteFrame(streamFrameWindowChange, payload)
	}
	if t.Session == nil {
		return errors.New("sshlib: terminal session is nil")
	}
	return t.Session.WindowChange(rows, cols)
}

// Wait waits for the remote session to finish.
func (t *Terminal) Wait() error {
	if t == nil {
		return errors.New("sshlib: terminal session is nil")
	}
	if t.waitCh != nil {
		return <-t.waitCh
	}
	if t.Session == nil {
		return errors.New("sshlib: terminal session is nil")
	}
	return t.Session.Wait()
}

// Close closes the terminal input pipe and session.
func (t *Terminal) Close() error {
	if t == nil {
		return nil
	}

	var err error
	t.closeOnce.Do(func() {
		if t.Stdin != nil {
			err = t.Stdin.Close()
		}
		if t.conn != nil {
			closeErr := t.conn.Close()
			if err == nil {
				err = closeErr
			}
		}
		if t.Session != nil {
			closeErr := t.Session.Close()
			if err == nil {
				err = closeErr
			}
		}
	})
	return err
}
