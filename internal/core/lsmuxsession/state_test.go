package lsmuxsession

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSocketPathUsesDefaultCacheLocation(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	path, err := resolveSocketPath("ops", "")
	if err != nil {
		t.Fatalf("resolveSocketPath returned error: %v", err)
	}

	wantDir := filepath.Join(cacheDir, "lssh", "lsmux")
	if filepath.Dir(path) != wantDir {
		t.Fatalf("resolveSocketPath directory = %q, want %q", filepath.Dir(path), wantDir)
	}
	if !strings.HasSuffix(path, ".sock") {
		t.Fatalf("resolveSocketPath path = %q, want .sock suffix", path)
	}
}

func TestResolveSocketPathExpandsHomeAndSessionName(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	path, err := resolveSocketPath("ops", "~/.cache/lssh/lsmux-<Name>.sock")
	if err != nil {
		t.Fatalf("resolveSocketPath returned error: %v", err)
	}

	want := filepath.Join(homeDir, ".cache", "lssh", "lsmux-ops.sock")
	if path != want {
		t.Fatalf("resolveSocketPath = %q, want %q", path, want)
	}
}

func TestSaveLoadAndListSessions(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	session := Session{
		Name:       "ops",
		PID:        1234,
		Network:    "unix",
		Address:    "/tmp/ops.sock",
		SocketPath: "/tmp/ops.sock",
	}
	if err := SaveSession(session); err != nil {
		t.Fatalf("SaveSession returned error: %v", err)
	}

	loaded, err := LoadSession("ops")
	if err != nil {
		t.Fatalf("LoadSession returned error: %v", err)
	}
	if loaded.Name != "ops" {
		t.Fatalf("LoadSession name = %q, want %q", loaded.Name, "ops")
	}
	if loaded.PID != 1234 {
		t.Fatalf("LoadSession pid = %d, want %d", loaded.PID, 1234)
	}

	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("ListSessions length = %d, want %d", len(sessions), 1)
	}
	if sessions[0].Name != "ops" {
		t.Fatalf("ListSessions first name = %q, want %q", sessions[0].Name, "ops")
	}
}
