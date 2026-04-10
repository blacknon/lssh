package conf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeOpenSSHIdentityFile(t *testing.T) {
	t.Parallel()

	if got := normalizeOpenSSHIdentityFile(""); got != "" {
		t.Fatalf("normalizeOpenSSHIdentityFile(\"\") = %q, want empty", got)
	}

	if got := normalizeOpenSSHIdentityFile("~/.ssh/identity"); got != "" {
		t.Fatalf("normalizeOpenSSHIdentityFile(default missing identity) = %q, want empty", got)
	}

	tempDir := t.TempDir()
	explicitKey := filepath.Join(tempDir, "demo_lssh_ed25519")
	if err := os.WriteFile(explicitKey, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write explicit key: %v", err)
	}

	if got := normalizeOpenSSHIdentityFile(explicitKey); got != explicitKey {
		t.Fatalf("normalizeOpenSSHIdentityFile(existing explicit key) = %q, want original path", got)
	}
}

func TestGenerateLSSHConfigFromOpenSSH(t *testing.T) {
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
		"    ForwardX11 yes",
		"    DynamicForward 11080",
		"",
		"Host *",
		"    ServerAliveInterval 30",
		"",
	}, "\n")
	if err := os.WriteFile(sshConfigPath, []byte(sshConfig), 0o600); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	data, err := GenerateLSSHConfigFromOpenSSH(sshConfigPath, "")
	if err != nil {
		t.Fatalf("GenerateLSSHConfigFromOpenSSH: %v", err)
	}

	output := string(data)
	if !strings.Contains(output, "[server.app]") {
		t.Fatalf("generated config missing host section: %s", output)
	}
	if strings.Contains(output, "[server."+sshConfigPath+":app]") {
		t.Fatalf("generated config should not prefix host with path: %s", output)
	}
	if !strings.Contains(output, `addr = "192.0.2.10"`) {
		t.Fatalf("generated config missing addr: %s", output)
	}
	if !strings.Contains(output, `user = "demo"`) {
		t.Fatalf("generated config missing user: %s", output)
	}
	if !strings.Contains(output, `key = "`+keyPath+`"`) {
		t.Fatalf("generated config missing key: %s", output)
	}
	if !strings.Contains(output, "x11 = true") {
		t.Fatalf("generated config missing x11 flag: %s", output)
	}
	if !strings.Contains(output, `dynamic_port_forward = "11080"`) {
		t.Fatalf("generated config missing dynamic forward: %s", output)
	}
}
