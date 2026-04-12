// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lspipe

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

type ExecOptions struct {
	Name    string
	Command string
	Hosts   []string
	Raw     bool
	Stdin   []byte
	Stdout  io.Writer
	Stderr  io.Writer
}

func Execute(opts ExecOptions) error {
	session, err := ResolveSession(opts.Name)
	if err != nil {
		return err
	}

	conn, err := dialSession(session)
	if err != nil {
		return err
	}
	defer conn.Close()

	if deadlineConn, ok := conn.(interface{ SetDeadline(time.Time) error }); ok {
		_ = deadlineConn.SetDeadline(time.Time{})
	}

	if err := json.NewEncoder(conn).Encode(Request{
		Action:  actionExec,
		Name:    session.Name,
		Command: strings.TrimSpace(opts.Command),
		Hosts:   opts.Hosts,
		Raw:     opts.Raw,
		Stdin:   opts.Stdin,
	}); err != nil {
		return err
	}

	if unixConn, ok := conn.(*net.UnixConn); ok {
		_ = unixConn.CloseWrite()
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	dec := json.NewDecoder(conn)
	exitCode := 0
	for {
		var event Event
		if err := dec.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		switch event.Type {
		case "stdout":
			if _, err := stdout.Write(event.Data); err != nil {
				return err
			}
		case "stderr":
			if _, err := stderr.Write(event.Data); err != nil {
				return err
			}
		case "error":
			return errors.New(event.Message)
		case "done":
			exitCode = event.ExitCode
		}
	}

	if exitCode != 0 {
		return fmt.Errorf("lspipe command failed with exit code %d", exitCode)
	}
	return nil
}
