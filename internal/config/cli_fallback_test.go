package conf

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWithFallbackNonInteractiveShowsGuidance(t *testing.T) {
	origInteractive, origGenerate, origRead := isInteractivePromptFn, generateConfigFromOpenSSHFn, readConfigFn
	t.Cleanup(func() {
		isInteractivePromptFn, generateConfigFromOpenSSHFn, readConfigFn = origInteractive, origGenerate, origRead
	})

	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "config"), []byte("Host app\n  HostName 192.0.2.10\n"), 0o600); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	isInteractivePromptFn = func() bool { return false }
	generateConfigFromOpenSSHFn = func(path, command string) ([]byte, error) {
		t.Fatal("generate should not be called in non-interactive mode")
		return nil, nil
	}
	readConfigFn = func(confPath string) (Config, error) { return Config{}, nil }

	var stderr bytes.Buffer
	_, err := ReadWithFallback(filepath.Join(home, ".lssh.toml"), &stderr)
	if err != nil {
		t.Fatalf("ReadWithFallback() error = %v", err)
	}
	if !strings.Contains(stderr.String(), "run `lssh --generate-lssh-conf") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestReadWithFallbackInteractiveCreatesAndReloads(t *testing.T) {
	origInteractive, origGenerate, origPrompt, origRead := isInteractivePromptFn, generateConfigFromOpenSSHFn, promptYesNoFn, readConfigFn
	t.Cleanup(func() {
		isInteractivePromptFn, generateConfigFromOpenSSHFn, promptYesNoFn, readConfigFn = origInteractive, origGenerate, origPrompt, origRead
	})

	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sshConfigPath := filepath.Join(home, ".ssh", "config")
	if err := os.WriteFile(sshConfigPath, []byte("Host app\n  HostName 192.0.2.10\n"), 0o600); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	isInteractivePromptFn = func() bool { return true }
	promptYesNoFn = func(stderr io.Writer, message string, defaultYes bool) (bool, error) {
		return true, nil
	}
	generateConfigFromOpenSSHFn = func(path, command string) ([]byte, error) {
		return []byte("[server.app]\naddr = \"192.0.2.10\"\nuser = \"demo\"\npass = \"secret\"\n"), nil
	}
	readConfigFn = func(confPath string) (Config, error) {
		return Config{Server: map[string]ServerConfig{
			"app": {Addr: "192.0.2.10", User: "demo", Pass: "secret"},
		}}, nil
	}

	confPath := filepath.Join(home, ".lssh.toml")
	var stderr bytes.Buffer
	cfg, err := ReadWithFallback(confPath, &stderr)
	if err != nil {
		t.Fatalf("ReadWithFallback() error = %v", err)
	}
	if cfg.Server["app"].Addr != "192.0.2.10" {
		t.Fatalf("cfg = %#v", cfg.Server["app"])
	}
	if _, err := os.Stat(confPath); err != nil {
		t.Fatalf("config not created: %v", err)
	}
}
