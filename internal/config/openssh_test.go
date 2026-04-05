package conf

import "testing"

func TestNormalizeOpenSSHIdentityFile(t *testing.T) {
	t.Parallel()

	if got := normalizeOpenSSHIdentityFile(""); got != "" {
		t.Fatalf("normalizeOpenSSHIdentityFile(\"\") = %q, want empty", got)
	}

	if got := normalizeOpenSSHIdentityFile("~/.ssh/identity"); got != "" {
		t.Fatalf("normalizeOpenSSHIdentityFile(default missing identity) = %q, want empty", got)
	}

	if got := normalizeOpenSSHIdentityFile("~/.ssh/demo_lssh_ed25519"); got != "~/.ssh/demo_lssh_ed25519" {
		t.Fatalf("normalizeOpenSSHIdentityFile(existing explicit key) = %q, want original path", got)
	}
}
