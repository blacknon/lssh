package apputil

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNotifyBackgroundReadyOnlyInDaemonMode(t *testing.T) {
	origDaemon := os.Getenv("_TEST_DAEMON")
	origReady := os.Getenv("_TEST_READY")
	t.Cleanup(func() {
		_ = os.Setenv("_TEST_DAEMON", origDaemon)
		_ = os.Setenv("_TEST_READY", origReady)
	})

	if err := os.Setenv("_TEST_DAEMON", "0"); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	if err := os.Setenv("_TEST_READY", filepath.Join(t.TempDir(), "ready")); err != nil {
		t.Fatalf("Setenv: %v", err)
	}

	NotifyBackgroundReady("_TEST_DAEMON", "_TEST_READY")
}

func TestNotifyBackgroundReadyWritesReadyFile(t *testing.T) {
	origDaemon := os.Getenv("_TEST_DAEMON")
	origReady := os.Getenv("_TEST_READY")
	t.Cleanup(func() {
		_ = os.Setenv("_TEST_DAEMON", origDaemon)
		_ = os.Setenv("_TEST_READY", origReady)
	})

	readyPath := filepath.Join(t.TempDir(), "ready")
	if err := os.Setenv("_TEST_DAEMON", "1"); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	if err := os.Setenv("_TEST_READY", readyPath); err != nil {
		t.Fatalf("Setenv: %v", err)
	}

	NotifyBackgroundReady("_TEST_DAEMON", "_TEST_READY")

	data, err := os.ReadFile(readyPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "OK\n" {
		t.Fatalf("ready file = %q, want OK", string(data))
	}
}

func TestWaitForBackgroundReadyFile(t *testing.T) {
	readyPath := filepath.Join(t.TempDir(), "ready")
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(readyPath, []byte("OK\n"), 0o600)
	}()

	if err := WaitForBackgroundReadyFile(readyPath, time.Second); err != nil {
		t.Fatalf("WaitForBackgroundReadyFile() error = %v", err)
	}
}
