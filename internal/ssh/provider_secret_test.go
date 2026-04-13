package ssh

import (
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
