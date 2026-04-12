package lspipe

import (
	"strings"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeSessionName(t *testing.T) {
	if got := normalizeSessionName(""); got != DefaultSessionName {
		t.Fatalf("normalizeSessionName(empty) = %q", got)
	}
	if got := normalizeSessionName(" prod "); got != "prod" {
		t.Fatalf("normalizeSessionName(trimmed) = %q", got)
	}
}

func TestSaveAndLoadSession(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	session := Session{
		Name:  "prod",
		Hosts: []string{"web02", "web01"},
		PID:   42,
	}
	if err := SaveSession(session); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	loaded, err := LoadSession("prod")
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if len(loaded.Hosts) != 2 || loaded.Hosts[0] != "web01" || loaded.Hosts[1] != "web02" {
		t.Fatalf("LoadSession() hosts = %#v", loaded.Hosts)
	}

	if err := RemoveSession("prod"); err != nil {
		t.Fatalf("RemoveSession() error = %v", err)
	}

	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("ListSessions() = %#v, want empty", sessions)
	}

	if _, err := os.Stat(cacheDir); err != nil {
		t.Fatalf("cache dir missing: %v", err)
	}
}

func TestListenerSpecCreatesStateDir(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	network, address, err := listenerSpec("prod")
	if err != nil {
		t.Fatalf("listenerSpec() error = %v", err)
	}
	if network == "unix" {
		if _, err := os.Stat(filepath.Dir(address)); err != nil {
			t.Fatalf("listenerSpec() did not create socket dir: %v", err)
		}
	}
}

func TestResolveSessionMarksMissingAddressAsStale(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	if err := SaveSession(Session{
		Name:  "prod",
		Hosts: []string{"web01"},
		PID:   42,
	}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	session, err := ResolveSession("prod")
	if err == nil {
		t.Fatal("ResolveSession() error = nil, want stale session error")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Fatalf("ResolveSession() error = %q, want stale message", err)
	}
	if !session.AliveChecked || !session.Stale {
		t.Fatalf("ResolveSession() = %#v, want checked stale session", session)
	}
}
