package conf

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blacknon/lssh/internal/providerapi"
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

func TestReadInventoryProvidersFailOpenLogsError(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-inventory")
	script := `#!/bin/sh
cat >/dev/null
printf '%s' '{"version":"v1","error":{"message":"inventory exploded"}}'
`
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	cfg := Config{
		Providers: ProvidersConfig{Paths: []string{providerPath}, FailOpen: true},
		Server:    map[string]ServerConfig{},
		Provider: map[string]map[string]interface{}{
			"aws": {
				"plugin":       "lssh-provider-fake-inventory",
				"capabilities": []interface{}{"inventory"},
			},
		},
	}

	var logbuf bytes.Buffer
	originalWriter := log.Writer()
	originalFlags := log.Flags()
	log.SetOutput(&logbuf)
	log.SetFlags(0)
	defer log.SetOutput(originalWriter)
	defer log.SetFlags(originalFlags)

	if err := cfg.ReadInventoryProviders(); err != nil {
		t.Fatalf("ReadInventoryProviders() error = %v", err)
	}

	got := logbuf.String()
	if !strings.Contains(got, `provider "aws" inventory failed but fail_open=true: provider "aws": inventory exploded`) {
		t.Fatalf("log = %q", got)
	}
}

func TestCallProviderReturnsJSONErrorOnExitStatusOne(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-inventory")
	script := `#!/bin/sh
cat >/dev/null
printf '%s' '{"version":"v1","error":{"message":"inventory exploded"}}'
exit 1
`
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	cfg := Config{
		Providers: ProvidersConfig{Paths: []string{providerPath}},
		Provider: map[string]map[string]interface{}{
			"aws": {
				"plugin":       "lssh-provider-fake-inventory",
				"capabilities": []interface{}{"inventory"},
			},
		},
	}

	var result providerapi.InventoryListResult
	err := cfg.callProvider("aws", providerapi.MethodInventoryList, providerapi.InventoryListParams{
		Provider: "aws",
		Config:   cfg.Provider["aws"],
	}, &result)
	if err == nil {
		t.Fatal("callProvider() error = nil")
	}
	if !strings.Contains(err.Error(), `provider "aws": inventory exploded`) {
		t.Fatalf("error = %q", err)
	}
}

func TestCallProviderWritesDebugLog(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-inventory")
	debugLogPath := filepath.Join(dir, "logs", "provider-debug.log")
	script := `#!/bin/sh
cat >/dev/null
printf '%s' '{"version":"v1","result":{"servers":[{"name":"aws:web-1","config":{"addr":"10.0.0.10"}}]}}'
printf '%s' 'debug note' >&2
`
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	cfg := Config{
		Providers: ProvidersConfig{Paths: []string{providerPath}, DebugLog: debugLogPath},
		Provider: map[string]map[string]interface{}{
			"aws": {
				"plugin":       "lssh-provider-fake-inventory",
				"capabilities": []interface{}{"inventory"},
			},
		},
	}

	var result providerapi.InventoryListResult
	if err := cfg.callProvider("aws", providerapi.MethodInventoryList, providerapi.InventoryListParams{
		Provider: "aws",
		Config:   cfg.Provider["aws"],
	}, &result); err != nil {
		t.Fatalf("callProvider() error = %v", err)
	}

	data, err := os.ReadFile(debugLogPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	got := string(data)
	if !strings.Contains(got, "provider=aws method=inventory.list") {
		t.Fatalf("debug log missing header: %q", got)
	}
	if !strings.Contains(got, `"method":"inventory.list"`) {
		t.Fatalf("debug log missing request: %q", got)
	}
	if !strings.Contains(got, `"servers":[{"name":"aws:web-1"`) {
		t.Fatalf("debug log missing stdout: %q", got)
	}
	if !strings.Contains(got, "stderr=debug note") {
		t.Fatalf("debug log missing stderr: %q", got)
	}
}

func TestReadInventoryProvidersDoesNotLeakProxmoxAPISettingsIntoSSHConfig(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "provider-inventory-proxmox")
	script := `#!/bin/sh
cat >/dev/null
printf '%s' '{"version":"v1","result":{"servers":[{"name":"pve:node:vm1","config":{"addr":"vm1.example.local"}}]}}'
`
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	cfg := Config{
		Providers: ProvidersConfig{Paths: []string{providerPath}},
		Server:    map[string]ServerConfig{},
		Provider: map[string]map[string]interface{}{
			"proxmox": {
				"plugin":       "provider-inventory-proxmox",
				"capabilities": []interface{}{"inventory"},
				"host":         "sv-pve.blckn",
				"port":         "8006",
				"username":     "root@pam",
				"user":         "provider-user-should-not-leak",
			},
		},
	}

	if err := cfg.ReadInventoryProviders(); err != nil {
		t.Fatalf("ReadInventoryProviders() error = %v", err)
	}

	server, ok := cfg.Server["pve:node:vm1"]
	if !ok {
		t.Fatalf("inventory server not loaded: %#v", cfg.Server)
	}
	if server.Port != "" {
		t.Fatalf("port = %q, want empty", server.Port)
	}
	if server.User != "" {
		t.Fatalf("user = %q, want empty", server.User)
	}
	if server.Addr != "vm1.example.local" {
		t.Fatalf("addr = %q", server.Addr)
	}
}

func TestApplyProviderInventoryMatchesNoteTemplate(t *testing.T) {
	base := ServerConfig{Note: "base-note"}
	matches := []providerInventoryMatch{
		{
			Name:       "append",
			When:       providerInventoryMatchWhen{MetaIn: []string{"node=sv-pve01"}},
			NoteAppend: " [${provider}:${meta:node}:${meta:status}]",
		},
		{
			Name:         "rewrite",
			When:         providerInventoryMatchWhen{MetaIn: []string{"type=qemu"}},
			NoteTemplate: "${note} -> ${server}",
		},
	}

	got := applyProviderInventoryMatches("proxmox", "pve:sv-pve01:vm1", map[string]string{
		"node":   "sv-pve01",
		"status": "running",
		"type":   "qemu",
	}, base, matches)

	want := "base-note [proxmox:sv-pve01:running] -> pve:sv-pve01:vm1"
	if got.Note != want {
		t.Fatalf("note = %q, want %q", got.Note, want)
	}
}

func TestActiveProvidersHonorsWhen(t *testing.T) {
	originalDetect := detectMatchContext
	defer func() { detectMatchContext = originalDetect }()
	detectMatchContext = func(reqs matchRequirements) matchContext {
		return matchContext{OS: "darwin"}
	}

	cfg := Config{
		Provider: map[string]map[string]interface{}{
			"proxmox": {
				"plugin":       "provider-inventory-proxmox",
				"capabilities": []interface{}{"inventory"},
				"when": map[string]interface{}{
					"os_in": []interface{}{"darwin"},
				},
			},
			"aws": {
				"plugin":       "provider-inventory-aws-ec2",
				"capabilities": []interface{}{"inventory"},
				"when": map[string]interface{}{
					"os_in": []interface{}{"linux"},
				},
			},
		},
	}

	got := cfg.activeProviders()
	if len(got) != 1 || got[0].name != "proxmox" {
		t.Fatalf("activeProviders() = %#v", got)
	}
}

func TestValidateProviderWhensRejectsInvalidCIDR(t *testing.T) {
	cfg := Config{
		Provider: map[string]map[string]interface{}{
			"proxmox": {
				"when": map[string]interface{}{
					"local_ip_in": []interface{}{"not-a-cidr"},
				},
			},
		},
	}

	_, err := cfg.validateProviderWhens()
	if err == nil || !strings.Contains(err.Error(), `server.provider.match.proxmox.when.local_ip_in`) {
		t.Fatalf("validateProviderWhens() error = %v", err)
	}
}
