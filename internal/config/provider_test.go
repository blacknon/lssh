package conf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadInventoryProviders(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-inventory")
	script := `#!/bin/sh
cat >/dev/null
printf '%s' '{"version":"v1","result":{"servers":[{"name":"aws:web-1","config":{"addr":"10.0.0.10","note":"from provider"}}]}}'
`
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	configPath := filepath.Join(dir, "lssh.toml")
	body := `
[providers]
paths = ["` + providerPath + `"]

[provider.aws]
plugin = "lssh-provider-fake-inventory"
capabilities = ["inventory"]

[provider.aws.match.web]
name_in = ["aws:web-*"]
user = "ubuntu"
key = "~/.ssh/web.pem"
`
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := Read(configPath)
	server, ok := cfg.Server["aws:web-1"]
	if !ok {
		t.Fatalf("inventory server not loaded: %#v", cfg.Server)
	}
	if server.User != "ubuntu" {
		t.Fatalf("user = %q, want ubuntu", server.User)
	}
	if server.Key != "~/.ssh/web.pem" {
		t.Fatalf("key = %q", server.Key)
	}
	if server.Addr != "10.0.0.10" {
		t.Fatalf("addr = %q", server.Addr)
	}
}

func TestReadInventoryProvidersMatchByMeta(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-inventory")
	script := `#!/bin/sh
cat >/dev/null
printf '%s' '{"version":"v1","result":{"servers":[{"name":"aws:app-1","config":{"addr":"10.0.0.11"},"meta":{"tag.Name":"app-1","tag.Role":"web","region":"ap-northeast-1"}}]}}'
`
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	configPath := filepath.Join(dir, "lssh.toml")
	body := `
[providers]
paths = ["` + providerPath + `"]

[provider.aws]
plugin = "lssh-provider-fake-inventory"
capabilities = ["inventory"]

[provider.aws.match.web]
meta_in = ["tag.Role=web", "region=ap-northeast-1"]
user = "ec2-user"
key = "~/.ssh/aws-web.pem"
`
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := Read(configPath)
	server := cfg.Server["aws:app-1"]
	if server.User != "ec2-user" {
		t.Fatalf("user = %q", server.User)
	}
	if server.Key != "~/.ssh/aws-web.pem" {
		t.Fatalf("key = %q", server.Key)
	}
}

func TestResolveSecretRef(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-secret")
	script := `#!/bin/sh
cat >/dev/null
printf '%s' '{"version":"v1","result":{"value":"super-secret"}}'
`
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	cfg := Config{
		Providers: ProvidersConfig{Paths: []string{providerPath}},
		Provider: map[string]map[string]interface{}{
			"onepassword": {
				"plugin":       "lssh-provider-fake-secret",
				"capabilities": []interface{}{"secret"},
			},
		},
	}

	value, err := cfg.ResolveSecretRef("onepassword:op://vault/item/field", "demo", "pass")
	if err != nil {
		t.Fatalf("ResolveSecretRef() error = %v", err)
	}
	if value != "super-secret" {
		t.Fatalf("ResolveSecretRef() = %q", value)
	}
}
