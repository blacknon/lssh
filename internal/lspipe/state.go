// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lspipe

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const DefaultSessionName = "default"

type HostHealth struct {
	Connected bool   `json:"connected"`
	Error     string `json:"error,omitempty"`
}

type Session struct {
	Name         string                `json:"name"`
	Hosts        []string              `json:"hosts"`
	PID          int                   `json:"pid"`
	Network      string                `json:"network"`
	Address      string                `json:"address"`
	ConfigPath   string                `json:"config_path"`
	CreatedAt    time.Time             `json:"created_at"`
	LastUsedAt   time.Time             `json:"last_used_at"`
	HostHealth   map[string]HostHealth `json:"host_health,omitempty"`
	Stale        bool                  `json:"-"`
	AliveChecked bool                  `json:"-"`
}

func stateDir() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CACHE_HOME")); xdg != "" {
		return filepath.Join(xdg, "lssh", "lspipe"), nil
	}

	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "lssh", "lspipe"), nil
}

func sessionFilePath(name string) (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}

	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(name))))
	filename := hex.EncodeToString(sum[:]) + ".json"
	return filepath.Join(dir, filename), nil
}

func socketFilePath(name string) (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}

	sum := sha1.Sum([]byte("sock:" + strings.ToLower(strings.TrimSpace(name))))
	filename := hex.EncodeToString(sum[:]) + ".sock"
	return filepath.Join(dir, filename), nil
}

func normalizeSessionName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return DefaultSessionName
	}
	return name
}

func normalizeSession(session Session) Session {
	session.Name = normalizeSessionName(session.Name)
	session.Hosts = append([]string(nil), session.Hosts...)
	sort.Strings(session.Hosts)
	if session.HostHealth == nil {
		session.HostHealth = map[string]HostHealth{}
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}
	if session.LastUsedAt.IsZero() {
		session.LastUsedAt = session.CreatedAt
	}
	return session
}

func SaveSession(session Session) error {
	session = normalizeSession(session)

	dir, err := stateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	path, err := sessionFilePath(session.Name)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

func LoadSession(name string) (Session, error) {
	name = normalizeSessionName(name)
	path, err := sessionFilePath(name)
	if err != nil {
		return Session{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Session{}, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return Session{}, err
	}
	return normalizeSession(session), nil
}

func RemoveSession(name string) error {
	name = normalizeSessionName(name)

	path, err := sessionFilePath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	sock, err := socketFilePath(name)
	if err == nil {
		if err := os.Remove(sock); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

func ListSessions() ([]Session, error) {
	dir, err := stateDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Session{}, nil
		}
		return nil, err
	}

	sessions := make([]Session, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}
		sessions = append(sessions, normalizeSession(session))
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Name < sessions[j].Name
	})

	return sessions, nil
}

func TouchSession(name string) error {
	session, err := LoadSession(name)
	if err != nil {
		return err
	}
	session.LastUsedAt = time.Now()
	return SaveSession(session)
}

func FormatSessionSummary(session Session) string {
	status := "alive"
	if session.AliveChecked && session.Stale {
		status = "stale"
	}

	return fmt.Sprintf(
		"%s\t%s\thosts=%d\tpid=%d\tlast_used=%s",
		session.Name,
		status,
		len(session.Hosts),
		session.PID,
		session.LastUsedAt.Format(time.RFC3339),
	)
}

func SessionExists(name string) bool {
	_, err := LoadSession(name)
	return err == nil
}
