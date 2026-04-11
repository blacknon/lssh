package lspipe

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBuildFIFOEndpoints(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	session := Session{
		Name:       "prod",
		Hosts:      []string{"web02", "web01"},
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
	}

	endpoints, baseDir, err := BuildFIFOEndpoints(session, "ops")
	if err != nil {
		t.Fatalf("BuildFIFOEndpoints() error = %v", err)
	}
	if len(endpoints) != 3 {
		t.Fatalf("BuildFIFOEndpoints() len = %d, want 3", len(endpoints))
	}
	if filepath.Base(baseDir) != "ops" {
		t.Fatalf("baseDir = %q", baseDir)
	}
	if filepath.Base(endpoints[0].CmdPath) != "all.cmd" {
		t.Fatalf("all cmd path = %q", endpoints[0].CmdPath)
	}
	if filepath.Base(endpoints[1].OutPath) != "web01.out" {
		t.Fatalf("web01 out path = %q", endpoints[1].OutPath)
	}
}
