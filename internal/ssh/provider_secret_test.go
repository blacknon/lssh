package ssh

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
)

func TestCreateAuthMethodMapWithPassRef(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-secret")
	script := `#!/bin/sh
cat >/dev/null
printf '%s' '{"version":"v1","result":{"value":"resolved-pass"}}'
`
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	run := &Run{
		ServerList: []string{"demo"},
		Conf: conf.Config{
			Providers: conf.ProvidersConfig{Paths: []string{dir}},
			Provider: map[string]map[string]interface{}{
				"onepassword": {
					"plugin":       "fake-secret",
					"capabilities": []interface{}{"secret"},
				},
			},
			Server: map[string]conf.ServerConfig{
				"demo": {
					Addr:    "127.0.0.1",
					User:    "demo",
					PassRef: "onepassword:op://vault/item/password",
				},
			},
		},
	}

	run.CreateAuthMethodMap()
	if len(run.serverAuthMethodMap["demo"]) == 0 {
		t.Fatalf("expected auth methods for pass_ref")
	}
}

func TestCreateAuthMethodMapWithKeyRefDoesNotLeaveTempFile(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-secret")
	keyData, err := os.ReadFile(filepath.Join("..", "..", "demo", "client", "home", ".ssh", "demo_lssh_ed25519"))
	if err != nil {
		t.Fatalf("read demo key: %v", err)
	}
	responsePath := filepath.Join(dir, "provider-response.json")
	responseBytes, err := json.Marshal(map[string]interface{}{
		"version": "v1",
		"result": map[string]string{
			"value": string(keyData),
		},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(responsePath, responseBytes, 0o600); err != nil {
		t.Fatalf("write response: %v", err)
	}

	script := "#!/bin/sh\ncat >/dev/null\ncat " + responsePath + "\n"
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	before, err := filepath.Glob(filepath.Join(os.TempDir(), "lssh-provider-secret-*"))
	if err != nil {
		t.Fatalf("Glob(before) error = %v", err)
	}

	run := &Run{
		ServerList: []string{"demo"},
		Conf: conf.Config{
			Providers: conf.ProvidersConfig{Paths: []string{dir}},
			Provider: map[string]map[string]interface{}{
				"onepassword": {
					"plugin":       "fake-secret",
					"capabilities": []interface{}{"secret"},
				},
			},
			Server: map[string]conf.ServerConfig{
				"demo": {
					Addr:   "127.0.0.1",
					User:   "demo",
					KeyRef: "onepassword:op://vault/item/privatekey",
				},
			},
		},
	}

	run.CreateAuthMethodMap()
	if len(run.serverAuthMethodMap["demo"]) == 0 {
		t.Fatalf("expected auth methods for key_ref")
	}

	after, err := filepath.Glob(filepath.Join(os.TempDir(), "lssh-provider-secret-*"))
	if err != nil {
		t.Fatalf("Glob(after) error = %v", err)
	}

	if len(after) != len(before) {
		t.Fatalf("temp files changed: before=%d after=%d", len(before), len(after))
	}
}

func TestCreateAuthMethodMapBlocksServerOnAuthPending(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-secret")
	script := `#!/bin/sh
cat >/dev/null
printf '%s' '{"version":"v1","error":{"code":"auth_pending","message":"Touch ID authentication required"}}'
exit 1
`
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	run := &Run{
		ServerList: []string{"demo"},
		Conf: conf.Config{
			Providers: conf.ProvidersConfig{Paths: []string{dir}},
			Provider: map[string]map[string]interface{}{
				"onepassword": {
					"plugin":       "fake-secret",
					"capabilities": []interface{}{"secret"},
				},
			},
			Server: map[string]conf.ServerConfig{
				"demo": {
					Addr:    "127.0.0.1",
					User:    "demo",
					PassRef: "onepassword:op://vault/item/password",
					Passes:  []string{"fallback-should-not-be-used"},
				},
			},
		},
	}

	run.CreateAuthMethodMap()
	if len(run.serverAuthMethodMap["demo"]) != 0 {
		t.Fatalf("expected no auth methods when auth is pending, got %d", len(run.serverAuthMethodMap["demo"]))
	}
}

func TestBuildControlPersistAuthMethodsFromConfigWithKeyRefKeepsResolvedTempFile(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-secret")
	keyData, err := os.ReadFile(filepath.Join("..", "..", "demo", "client", "home", ".ssh", "demo_lssh_ed25519"))
	if err != nil {
		t.Fatalf("read demo key: %v", err)
	}
	responsePath := filepath.Join(dir, "provider-response.json")
	responseBytes, err := json.Marshal(map[string]interface{}{
		"version": "v1",
		"result": map[string]string{
			"value": string(keyData),
		},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(responsePath, responseBytes, 0o600); err != nil {
		t.Fatalf("write response: %v", err)
	}

	script := "#!/bin/sh\ncat >/dev/null\ncat " + responsePath + "\n"
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	run := &Run{
		Conf: conf.Config{
			Providers: conf.ProvidersConfig{Paths: []string{dir}},
			Provider: map[string]map[string]interface{}{
				"onepassword": {
					"plugin":       "fake-secret",
					"capabilities": []interface{}{"secret"},
				},
			},
			Server: map[string]conf.ServerConfig{
				"demo": {
					Addr:   "127.0.0.1",
					User:   "demo",
					KeyRef: "onepassword:op://vault/item/privatekey",
				},
			},
		},
	}

	methods, err := run.buildControlPersistAuthMethodsFromConfig("demo", run.Conf.Server["demo"])
	if err != nil {
		t.Fatalf("buildControlPersistAuthMethodsFromConfig() error = %v", err)
	}
	if len(methods) == 0 {
		t.Fatal("expected control persist auth methods")
	}

	files, err := filepath.Glob(filepath.Join(os.TempDir(), "lssh-provider-secret-controlpersist-key-*"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected persistent temp key file")
	}

	found := false
	for _, name := range files {
		data, readErr := os.ReadFile(name)
		if readErr == nil && string(data) == string(keyData) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("resolved key temp file was not preserved")
	}
}
