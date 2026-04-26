package providerapi

import (
	"encoding/base64"
	"encoding/json"
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

	key := "LSSH_PROVIDERAPI_TEST_TOKEN"
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

func TestReadRequestFromEnv(t *testing.T) {
	t.Parallel()

	request := Request{
		Version: Version,
		Method:  MethodConnectorShell,
		Params: map[string]interface{}{
			"provider": "aws",
		},
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.Setenv(RequestEnvVar, base64.StdEncoding.EncodeToString(data)); err != nil {
		t.Fatalf("Setenv() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv(RequestEnvVar)
	})

	got, err := ReadRequest()
	if err != nil {
		t.Fatalf("ReadRequest() error = %v", err)
	}
	if got.Method != MethodConnectorShell {
		t.Fatalf("ReadRequest().Method = %q, want %q", got.Method, MethodConnectorShell)
	}
}

func TestWriteRuntimeResult(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "result.json")
	if err := os.Setenv(ResultEnvVar, path); err != nil {
		t.Fatalf("Setenv() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv(ResultEnvVar)
	})

	want := ConnectorExecResult{ExitCode: 23}
	if err := WriteRuntimeResult(want); err != nil {
		t.Fatalf("WriteRuntimeResult() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var got ConnectorExecResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.ExitCode != want.ExitCode {
		t.Fatalf("ExitCode = %d, want %d", got.ExitCode, want.ExitCode)
	}
}
