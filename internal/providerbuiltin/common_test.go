package providerbuiltin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigValueLiteral(t *testing.T) {
	t.Parallel()

	value, err := ResolveConfigValue(map[string]interface{}{
		"token": "literal-value",
	}, "token")
	if err != nil {
		t.Fatalf("ResolveConfigValue returned error: %v", err)
	}
	if value != "literal-value" {
		t.Fatalf("unexpected value: %q", value)
	}
}

func TestResolveConfigValueEnv(t *testing.T) {
	t.Parallel()

	key := "LSSH_PROVIDERBUILTIN_TEST_TOKEN"
	if err := os.Setenv(key, "env-value"); err != nil {
		t.Fatalf("Setenv failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv(key)
	})

	value, err := ResolveConfigValue(map[string]interface{}{
		"token_env": key,
	}, "token")
	if err != nil {
		t.Fatalf("ResolveConfigValue returned error: %v", err)
	}
	if value != "env-value" {
		t.Fatalf("unexpected value: %q", value)
	}
}

func TestResolveConfigValueSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "provider.env")
	content := "export OP_SERVICE_ACCOUNT_TOKEN=source-value\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	value, err := ResolveConfigValue(map[string]interface{}{
		"token_source":     path,
		"token_source_env": "OP_SERVICE_ACCOUNT_TOKEN",
	}, "token")
	if err != nil {
		t.Fatalf("ResolveConfigValue returned error: %v", err)
	}
	if value != "source-value" {
		t.Fatalf("unexpected value: %q", value)
	}
}

func TestExpandPaths(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir failed: %v", err)
	}

	got := ExpandPaths([]string{"~/.aws/config", "/tmp/aws-credentials", ""})
	want := []string{
		filepath.Join(home, ".aws", "config"),
		"/tmp/aws-credentials",
		"",
	}
	if len(got) != len(want) {
		t.Fatalf("len(ExpandPaths()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ExpandPaths()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
