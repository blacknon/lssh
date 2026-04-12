package conf

import (
	"os"
	"os/user"
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

func TestLoadOpenSSHConfigEntriesAppliesMatchUser(t *testing.T) {
	t.Parallel()

	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("current user: %v", err)
	}

	tmpDir := t.TempDir()
	sshConfigPath := filepath.Join(tmpDir, "config")
	sshConfig := strings.Join([]string{
		"Host app",
		"    HostName 192.0.2.10",
		"    User demo",
		"",
		"Match user demo originalhost app",
		"    Port 2200",
		"",
		"Match localuser " + currentUser.Username + " host 192.0.2.10",
		"    ForwardX11 yes",
		"",
	}, "\n")
	if err := os.WriteFile(sshConfigPath, []byte(sshConfig), 0o600); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	entries, err := loadOpenSSHConfigEntries(sshConfigPath, "")
	if err != nil {
		t.Fatalf("loadOpenSSHConfigEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	entry := entries[0]
	if entry.Host != "app" {
		t.Fatalf("entry.Host = %q, want app", entry.Host)
	}
	if entry.Config.Port != "2200" {
		t.Fatalf("entry.Config.Port = %q, want 2200", entry.Config.Port)
	}
	if !entry.Config.X11 {
		t.Fatalf("entry.Config.X11 = false, want true")
	}
}

func TestCurrentUsernameFallsBackToWindowsStyleEnv(t *testing.T) {
	t.Setenv("USER", "")
	t.Setenv("USERNAME", "demo-user")

	if got := currentUsername(); got != "demo-user" {
		t.Fatalf("currentUsername() = %q, want %q", got, "demo-user")
	}
}
