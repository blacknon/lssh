// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lspipe

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"
)

var dialSession = DialSession

const (
	actionPing = "ping"
	actionExec = "exec"
)

type Request struct {
	Action  string   `json:"action"`
	Name    string   `json:"name,omitempty"`
	Command string   `json:"command,omitempty"`
	Hosts   []string `json:"hosts,omitempty"`
	Raw     bool     `json:"raw,omitempty"`
	Stdin   []byte   `json:"stdin,omitempty"`
}

type Event struct {
	Type     string `json:"type"`
	Host     string `json:"host,omitempty"`
	Stream   string `json:"stream,omitempty"`
	Data     []byte `json:"data,omitempty"`
	Message  string `json:"message,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
}

func listenerSpec(name string) (network, address string, err error) {
	name = normalizeSessionName(name)
	if runtime.GOOS == "windows" {
		return "tcp", "127.0.0.1:0", nil
	}

	dir, err := stateDir()
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}

	path, err := socketFilePath(name)
	if err != nil {
		return "", "", err
	}
	return "unix", path, nil
}

func DialSession(session Session) (net.Conn, error) {
	session.Name = normalizeSessionName(session.Name)
	if session.Network == "" || session.Address == "" {
		return nil, errors.New("session does not have a dialable address")
	}

	return net.DialTimeout(session.Network, session.Address, 2*time.Second)
}

func PingSession(session Session) bool {
	conn, err := dialSession(session)
	if err != nil {
		return false
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(Request{Action: actionPing, Name: session.Name}); err != nil {
		return false
	}

	var event Event
	if err := dec.Decode(&event); err != nil {
		return false
	}

	return event.Type == "pong"
}

func MarkSessionAlive(session *Session) {
	session.AliveChecked = true
	session.Stale = !PingSession(*session)
}

func ResolveSession(name string) (Session, error) {
	session, err := LoadSession(name)
	if err != nil {
		return Session{}, err
	}

	MarkSessionAlive(&session)
	if session.Stale {
		return session, fmt.Errorf("lspipe session %q is stale; recreate it with lspipe --replace --name %s", session.Name, session.Name)
	}

	return session, nil
}
