package conf

import (
	"os"
	"path/filepath"
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
