package lsmuxsession

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const DefaultSessionName = "default"

type Session struct {
	Name         string    `json:"name"`
	PID          int       `json:"pid"`
	Network      string    `json:"network"`
	Address      string    `json:"address"`
	SocketPath   string    `json:"socket_path,omitempty"`
	ConfigPath   string    `json:"config_path,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	LastAttached time.Time `json:"last_attached_at"`
	Stale        bool      `json:"-"`
	AliveChecked bool      `json:"-"`
}

func stateDir() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CACHE_HOME")); xdg != "" {
		return filepath.Join(xdg, "lssh", "lsmux"), nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "lssh", "lsmux"), nil
}

func normalizeSessionName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return DefaultSessionName
	}
	return name
}

func sessionFilePath(name string) (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	sum := sha1.Sum([]byte(strings.ToLower(normalizeSessionName(name))))
	return filepath.Join(dir, hex.EncodeToString(sum[:])+".json"), nil
}

func defaultSocketFilePath(name string) (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	sum := sha1.Sum([]byte("sock:" + strings.ToLower(normalizeSessionName(name))))
	return filepath.Join(dir, hex.EncodeToString(sum[:])+".sock"), nil
}

func resolveSocketPath(name, configured string) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return defaultSocketFilePath(name)
	}
	configured = strings.ReplaceAll(configured, "<Name>", normalizeSessionName(name))
	if strings.HasPrefix(configured, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			if configured == "~" {
				configured = home
			} else if strings.HasPrefix(configured, "~/") {
				configured = filepath.Join(home, configured[2:])
			}
		}
	}
	return configured, nil
}

func SaveSession(session Session) error {
	session.Name = normalizeSessionName(session.Name)
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}
	if session.LastAttached.IsZero() {
		session.LastAttached = session.CreatedAt
	}
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
	session.Name = normalizeSessionName(session.Name)
	return session, nil
}

func RemoveSession(name string) error {
	session, _ := LoadSession(name)
	path, err := sessionFilePath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if runtime.GOOS != "windows" && strings.TrimSpace(session.SocketPath) != "" {
		if err := os.Remove(session.SocketPath); err != nil && !os.IsNotExist(err) {
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
		session.Name = normalizeSessionName(session.Name)
		sessions = append(sessions, session)
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].Name < sessions[j].Name })
	return sessions, nil
}

func FormatSessionSummary(session Session) string {
	status := "alive"
	if session.AliveChecked && session.Stale {
		status = "stale"
	}
	return fmt.Sprintf("%s\t%s\tpid=%d\tlast_attached=%s", session.Name, status, session.PID, session.LastAttached.Format(time.RFC3339))
}
