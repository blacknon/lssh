package conf

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleGenerateConfigMode(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "id_test")
	if err := os.WriteFile(keyPath, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	sshConfigPath := filepath.Join(tmpDir, "config")
	sshConfig := strings.Join([]string{
		"Host app",
		"    HostName 192.0.2.10",
		"    User demo",
		"    IdentityFile " + keyPath,
		"",
	}, "\n")
	if err := os.WriteFile(sshConfigPath, []byte(sshConfig), 0o600); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	var out bytes.Buffer
	handled, err := HandleGenerateConfigMode(sshConfigPath, &out)
	if err != nil {
		t.Fatalf("HandleGenerateConfigMode: %v", err)
	}
	if !handled {
		t.Fatalf("HandleGenerateConfigMode() = false, want true")
	}
	if !strings.Contains(out.String(), "[server.app]") {
		t.Fatalf("generated config missing host section: %s", out.String())
	}
}

func TestHandleGenerateConfigModeNoop(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	handled, err := HandleGenerateConfigMode("", &out)
	if err != nil {
		t.Fatalf("HandleGenerateConfigMode noop error: %v", err)
	}
	if handled {
		t.Fatalf("HandleGenerateConfigMode() = true, want false")
	}
	if out.Len() != 0 {
		t.Fatalf("HandleGenerateConfigMode wrote output for noop: %q", out.String())
	}
}
