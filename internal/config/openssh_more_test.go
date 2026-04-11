package conf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateLSSHConfigFromOpenSSHKeepsMultipleKeysAndCertificates(t *testing.T) {
	tmpDir := t.TempDir()
	key1 := filepath.Join(tmpDir, "id_one")
	key2 := filepath.Join(tmpDir, "id_two")
	cert1 := filepath.Join(tmpDir, "id_one-cert.pub")
	cert2 := filepath.Join(tmpDir, "id_two-cert.pub")
	for _, path := range []string{key1, key2, cert1, cert2} {
		if err := os.WriteFile(path, []byte("dummy"), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	sshConfigPath := filepath.Join(tmpDir, "config")
	sshConfig := strings.Join([]string{
		"Host app",
		"    HostName 192.0.2.10",
		"    User demo",
		"    IdentityFile " + key1,
		"    IdentityFile " + key2,
		"    CertificateFile " + cert1,
		"    CertificateFile " + cert2,
		"    DynamicForward 11080",
		"    ControlPersist 30s",
		"",
	}, "\n")
	if err := os.WriteFile(sshConfigPath, []byte(sshConfig), 0o600); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	data, err := GenerateLSSHConfigFromOpenSSH(sshConfigPath, "")
	if err != nil {
		t.Fatalf("GenerateLSSHConfigFromOpenSSH() error = %v", err)
	}

	out := string(data)
	for _, needle := range []string{
		`cert = "` + cert1 + `"`,
		`certkey = "` + key1 + `"`,
		`certs = ["` + cert2 + "::" + key2 + `"]`,
		`keys = ["` + key2 + `"]`,
		`dynamic_port_forward = "11080"`,
		`control_persist = 30`,
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("missing %q in output:\n%s", needle, out)
		}
	}
}
