// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"errors"
	"io"

	"golang.org/x/crypto/ssh"
)

// Terminal is a low-level interactive terminal session.
type Terminal struct {
	Session *ssh.Session
	Stdin   io.WriteCloser
	Stdout  io.Reader
	Stderr  io.Reader
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

// Resize updates the remote PTY size.
func (t *Terminal) Resize(cols, rows int) error {
	if t == nil || t.Session == nil {
		return errors.New("sshlib: terminal session is nil")
	}
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	return t.Session.WindowChange(rows, cols)
}

// Wait waits for the remote session to finish.
func (t *Terminal) Wait() error {
	if t == nil || t.Session == nil {
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
	if t.Stdin != nil {
		err = t.Stdin.Close()
	}
	if t.Session != nil {
		closeErr := t.Session.Close()
		if err == nil {
			err = closeErr
		}
	}
	return err
}
