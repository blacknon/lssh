package lsmuxsession

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Message struct {
	Type    string `json:"type"`
	Data    []byte `json:"data,omitempty"`
	Cols    int    `json:"cols,omitempty"`
	Rows    int    `json:"rows,omitempty"`
	Message string `json:"message,omitempty"`
}

func listenerSpec(name, socketPath string) (network, address, resolvedSocket string, err error) {
	name = normalizeSessionName(name)
	if runtime.GOOS == "windows" {
		return "tcp", "127.0.0.1:0", "", nil
	}
	dir, err := stateDir()
	if err != nil {
		return "", "", "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", "", err
	}
	resolvedSocket, err = resolveSocketPath(name, socketPath)
	if err != nil {
		return "", "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(resolvedSocket), 0o700); err != nil {
		return "", "", "", err
	}
	return "unix", resolvedSocket, resolvedSocket, nil
}

func DialSession(session Session) (net.Conn, error) {
	if session.Network == "" || session.Address == "" {
		return nil, errors.New("session does not have a dialable address")
	}
	return net.DialTimeout(session.Network, session.Address, 2*time.Second)
}

func PingSession(session Session) bool {
	conn, err := DialSession(session)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(Message{Type: "ping"}); err != nil {
		return false
	}
	var msg Message
	if err := dec.Decode(&msg); err != nil {
		return false
	}
	return msg.Type == "pong"
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
		return session, errors.New("lsmux session is stale")
	}
	return session, nil
}
