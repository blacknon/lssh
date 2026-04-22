package conf

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestReadInventoryProvidersMatchRespectsPriorityThenDeclarationOrderTOML(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-inventory")
	script := `#!/bin/sh
cat >/dev/null
printf '%s' '{"version":"v1","result":{"servers":[{"name":"aws:web-1","config":{"addr":"10.0.0.10"}}]}}'
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
user = "ubuntu"
key = "~/.ssh/test.pem"

[provider.aws.match.second]
priority = 1
name_in = ["aws:web-*"]
addr = "10.0.0.52"

[provider.aws.match.first]
priority = 1
name_in = ["aws:web-*"]
addr = "10.0.0.51"
`
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := Read(configPath)
	server, ok := cfg.Server["aws:web-1"]
	if !ok {
		t.Fatalf("inventory server not loaded: %#v", cfg.Server)
	}
	if server.Addr != "10.0.0.51" {
		t.Fatalf("addr = %q, want 10.0.0.51", server.Addr)
	}
}

func TestReadInventoryProvidersMatchRespectsPriorityThenDeclarationOrderYAML(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-inventory")
	script := `#!/bin/sh
cat >/dev/null
printf '%s' '{"version":"v1","result":{"servers":[{"name":"aws:web-1","config":{"addr":"10.0.0.10"}}]}}'
`
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	configPath := filepath.Join(dir, "lssh.yaml")
	body := `
providers:
  paths:
    - ` + providerPath + `
provider:
  aws:
    plugin: lssh-provider-fake-inventory
    capabilities:
      - inventory
    user: ubuntu
    key: ~/.ssh/test.pem
    match:
      second:
        priority: 1
        name_in:
          - aws:web-*
        addr: 10.0.0.52
      first:
        priority: 1
        name_in:
          - aws:web-*
        addr: 10.0.0.51
`
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := Read(configPath)
	server, ok := cfg.Server["aws:web-1"]
	if !ok {
		t.Fatalf("inventory server not loaded: %#v", cfg.Server)
	}
	if server.Addr != "10.0.0.51" {
		t.Fatalf("addr = %q, want 10.0.0.51", server.Addr)
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

func TestReadInventoryProvidersMatchSetsConnectorName(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-inventory")
	script := `#!/bin/sh
cat >/dev/null
printf '%s' '{"version":"v1","result":{"servers":[{"name":"aws:win-1","config":{"addr":"10.0.0.12"},"meta":{"platform":"windows"}}]}}'
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

[provider.aws.match.windows]
meta_in = ["platform=windows"]
connector_name = "winrm"
`
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := Read(configPath)
	server := cfg.Server["aws:win-1"]
	if server.ConnectorName != "winrm" {
		t.Fatalf("connector_name = %q, want winrm", server.ConnectorName)
	}
}

func TestServerUsesConnectorRespectsConnectorName(t *testing.T) {
	cfg := Config{
		Server: map[string]ServerConfig{
			"ssh-host":  {ConnectorName: "ssh"},
			"conn-host": {ConnectorName: "openssh"},
		},
	}

	if cfg.ServerUsesConnector("ssh-host") {
		t.Fatal("ServerUsesConnector(ssh-host) = true, want false")
	}
	if !cfg.ServerUsesConnector("conn-host") {
		t.Fatal("ServerUsesConnector(conn-host) = false, want true")
	}
}

func TestPrepareConnectorResolvesProviderByConnectorName(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-connector")
	script := `#!/bin/sh
payload="$(cat)"
case "$payload" in
  *'"method":"plugin.describe"'*)
    printf '%s' '{"version":"v1","result":{"name":"fake-connector","capabilities":["connector"],"connector_names":["openssh"],"methods":["plugin.describe","connector.prepare"]}}'
    ;;
  *'"method":"connector.prepare"'*)
    printf '%s' '{"version":"v1","result":{"supported":true,"plan":{"kind":"command","program":"ssh","args":["example.internal"],"details":{"connector":"openssh"}}}}'
    ;;
  *)
    printf '%s' '{"version":"v1","error":{"message":"unsupported"}}'
    exit 1
    ;;
esac
`
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	cfg := Config{
		Providers: ProvidersConfig{Paths: []string{providerPath}},
		Server: map[string]ServerConfig{
			"web01": {
				Addr:          "example.internal",
				User:          "demo",
				ConnectorName: "openssh",
			},
		},
		Provider: map[string]map[string]interface{}{
			"openssh": {
				"plugin":       "lssh-provider-fake-connector",
				"enabled":      true,
				"capabilities": []interface{}{"connector"},
			},
		},
	}

	result, err := cfg.PrepareConnector("web01", providerapi.ConnectorOperation{Name: "shell"})
	if err != nil {
		t.Fatalf("PrepareConnector() error = %v", err)
	}
	if !result.Supported {
		t.Fatal("PrepareConnector().Supported = false, want true")
	}
	if got := result.Plan.Details["connector"]; got != "openssh" {
		t.Fatalf("PrepareConnector().Plan.Details[connector] = %v, want openssh", got)
	}
}

func TestServerConnectorNameFallsBackToProviderDescriptor(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-connector")
	script := `#!/bin/sh
cat >/dev/null
printf '%s' '{"version":"v1","result":{"name":"fake-connector","capabilities":["connector"],"connector_names":["aws-ssm"],"methods":["plugin.describe"]}}'
`
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	cfg := Config{
		Providers: ProvidersConfig{Paths: []string{providerPath}},
		Server: map[string]ServerConfig{
			"aws:web-01": {ProviderName: "aws"},
		},
		Provider: map[string]map[string]interface{}{
			"aws": {
				"plugin":       "lssh-provider-fake-connector",
				"enabled":      true,
				"capabilities": []interface{}{"inventory", "connector"},
			},
		},
	}

	if got := cfg.ServerConnectorName("aws:web-01"); got != "aws-ssm" {
		t.Fatalf("ServerConnectorName() = %q, want aws-ssm", got)
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
	if !strings.Contains(got, `"name":"aws:web-1"`) {
		t.Fatalf("debug log missing stdout: %q", got)
	}
	if !strings.Contains(got, "stderr=debug note") {
		t.Fatalf("debug log missing stderr: %q", got)
	}
}

func TestCallProviderDebugLogRedactsSecretConfigAndResult(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "lssh-provider-fake-secret")
	debugLogPath := filepath.Join(dir, "logs", "provider-debug.log")
	script := `#!/bin/sh
cat >/dev/null
printf '%s' '{"version":"v1","result":{"value":"super-secret-password"}}'
`
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	cfg := Config{
		Providers: ProvidersConfig{Paths: []string{providerPath}, DebugLog: debugLogPath},
		Provider: map[string]map[string]interface{}{
			"onepassword": {
				"plugin":       "lssh-provider-fake-secret",
				"capabilities": []interface{}{"secret"},
				"token":        "ops_example_token",
				"auth_mode":    "service_account",
			},
		},
	}

	var result providerapi.SecretGetResult
	if err := cfg.callProvider("onepassword", providerapi.MethodSecretGet, providerapi.SecretGetParams{
		Provider: "onepassword",
		Config:   cfg.Provider["onepassword"],
		Ref:      "op://vault/item/password",
		Server:   "demo",
		Field:    "pass",
	}, &result); err != nil {
		t.Fatalf("callProvider() error = %v", err)
	}

	data, err := os.ReadFile(debugLogPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	got := string(data)
	if strings.Contains(got, "ops_example_token") {
		t.Fatalf("debug log leaked provider token: %q", got)
	}
	if strings.Contains(got, "super-secret-password") {
		t.Fatalf("debug log leaked secret value: %q", got)
	}
	if !strings.Contains(got, `"token":"\u003credacted\u003e"`) {
		t.Fatalf("debug log missing redacted token: %q", got)
	}
	if !strings.Contains(got, `"value":"\u003credacted\u003e"`) {
		t.Fatalf("debug log missing redacted secret result: %q", got)
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

func TestReadInventoryProvidersDoesNotLeakAzureAPISettingsIntoSSHConfig(t *testing.T) {
	dir := t.TempDir()
	providerPath := filepath.Join(dir, "provider-inventory-azure-compute")
	script := `#!/bin/sh
cat >/dev/null
printf '%s' '{"version":"v1","result":{"servers":[{"name":"azure:vm1","config":{"addr":"10.0.0.4"}}]}}'
`
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider: %v", err)
	}

	cfg := Config{
		Providers: ProvidersConfig{Paths: []string{providerPath}},
		Server:    map[string]ServerConfig{},
		Provider: map[string]map[string]interface{}{
			"azure": {
				"plugin":          "provider-inventory-azure-compute",
				"capabilities":    []interface{}{"inventory"},
				"subscription_id": "sub-1",
				"tenant_id":       "tenant-1",
				"client_id":       "client-1",
				"client_secret":   "secret-1",
				"resource_group":  "rg-demo",
				"user":            "provider-user-should-not-leak",
			},
		},
	}

	if err := cfg.ReadInventoryProviders(); err != nil {
		t.Fatalf("ReadInventoryProviders() error = %v", err)
	}

	server, ok := cfg.Server["azure:vm1"]
	if !ok {
		t.Fatalf("inventory server not loaded: %#v", cfg.Server)
	}
	if server.User != "" {
		t.Fatalf("user = %q, want empty", server.User)
	}
	if server.Addr != "10.0.0.4" {
		t.Fatalf("addr = %q, want %q", server.Addr, "10.0.0.4")
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
				"plugin":       "provider-mixed-aws-ec2",
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

func TestReadInventoryProvidersFetchesProvidersInParallel(t *testing.T) {
	dir := t.TempDir()
	providerPathA := filepath.Join(dir, "lssh-provider-fake-inventory-a")
	providerPathB := filepath.Join(dir, "lssh-provider-fake-inventory-b")

	script := `#!/bin/sh
sleep 0.3
cat >/dev/null
printf '%s' '{"version":"v1","result":{"servers":[{"name":"aws:web-1","config":{"addr":"10.0.0.10"}}]}}'
`
	if err := os.WriteFile(providerPathA, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider A: %v", err)
	}
	if err := os.WriteFile(providerPathB, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider B: %v", err)
	}

	cfg := Config{
		Providers: ProvidersConfig{Paths: []string{providerPathA, providerPathB}},
		Server:    map[string]ServerConfig{},
		Provider: map[string]map[string]interface{}{
			"aws-a": {
				"plugin":       "lssh-provider-fake-inventory-a",
				"capabilities": []interface{}{"inventory"},
			},
			"aws-b": {
				"plugin":       "lssh-provider-fake-inventory-b",
				"capabilities": []interface{}{"inventory"},
			},
		},
	}

	start := time.Now()
	if err := cfg.ReadInventoryProviders(); err != nil {
		t.Fatalf("ReadInventoryProviders() error = %v", err)
	}
	if elapsed := time.Since(start); elapsed >= 900*time.Millisecond {
		t.Fatalf("ReadInventoryProviders() took %v, want parallel execution under 900ms", elapsed)
	}
}

func TestReadInventoryProvidersMergesResultsDeterministicallyAfterParallelFetch(t *testing.T) {
	dir := t.TempDir()
	providerPathA := filepath.Join(dir, "lssh-provider-fake-inventory-a")
	providerPathB := filepath.Join(dir, "lssh-provider-fake-inventory-b")

	scriptA := `#!/bin/sh
sleep 0.3
cat >/dev/null
printf '%s' '{"version":"v1","result":{"servers":[{"name":"aws:web-1","config":{"addr":"10.0.0.10","note":"from-a"}}]}}'
`
	scriptB := `#!/bin/sh
sleep 0.1
cat >/dev/null
printf '%s' '{"version":"v1","result":{"servers":[{"name":"aws:web-1","config":{"addr":"10.0.0.20","note":"from-b"}}]}}'
`
	if err := os.WriteFile(providerPathA, []byte(scriptA), 0o755); err != nil {
		t.Fatalf("write provider A: %v", err)
	}
	if err := os.WriteFile(providerPathB, []byte(scriptB), 0o755); err != nil {
		t.Fatalf("write provider B: %v", err)
	}

	cfg := Config{
		Providers: ProvidersConfig{Paths: []string{providerPathA, providerPathB}},
		Server:    map[string]ServerConfig{},
		Provider: map[string]map[string]interface{}{
			"aws-a": {
				"plugin":       "lssh-provider-fake-inventory-a",
				"capabilities": []interface{}{"inventory"},
			},
			"aws-b": {
				"plugin":       "lssh-provider-fake-inventory-b",
				"capabilities": []interface{}{"inventory"},
			},
		},
	}

	if err := cfg.ReadInventoryProviders(); err != nil {
		t.Fatalf("ReadInventoryProviders() error = %v", err)
	}

	server, ok := cfg.Server["aws:web-1"]
	if !ok {
		t.Fatalf("inventory server not loaded: %#v", cfg.Server)
	}
	if server.Addr != "10.0.0.20" {
		t.Fatalf("addr = %q, want 10.0.0.20", server.Addr)
	}
	if server.Note != "from-b" {
		t.Fatalf("note = %q, want from-b", server.Note)
	}
	if server.ProviderName != "aws-b" {
		t.Fatalf("provider = %q, want aws-b", server.ProviderName)
	}
}

func TestReadInventoryProvidersHonorsMaxParallel(t *testing.T) {
	dir := t.TempDir()
	providerPathA := filepath.Join(dir, "lssh-provider-fake-inventory-a")
	providerPathB := filepath.Join(dir, "lssh-provider-fake-inventory-b")
	providerPathC := filepath.Join(dir, "lssh-provider-fake-inventory-c")

	script := `#!/bin/sh
sleep 0.3
cat >/dev/null
printf '%s' '{"version":"v1","result":{"servers":[{"name":"aws:web-1","config":{"addr":"10.0.0.10"}}]}}'
`
	if err := os.WriteFile(providerPathA, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider A: %v", err)
	}
	if err := os.WriteFile(providerPathB, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider B: %v", err)
	}
	if err := os.WriteFile(providerPathC, []byte(script), 0o755); err != nil {
		t.Fatalf("write provider C: %v", err)
	}

	cfg := Config{
		Providers: ProvidersConfig{
			Paths:       []string{providerPathA, providerPathB, providerPathC},
			MaxParallel: 1,
		},
		Server: map[string]ServerConfig{},
		Provider: map[string]map[string]interface{}{
			"aws-a": {
				"plugin":       "lssh-provider-fake-inventory-a",
				"capabilities": []interface{}{"inventory"},
			},
			"aws-b": {
				"plugin":       "lssh-provider-fake-inventory-b",
				"capabilities": []interface{}{"inventory"},
			},
			"aws-c": {
				"plugin":       "lssh-provider-fake-inventory-c",
				"capabilities": []interface{}{"inventory"},
			},
		},
	}

	start := time.Now()
	if err := cfg.ReadInventoryProviders(); err != nil {
		t.Fatalf("ReadInventoryProviders() error = %v", err)
	}
	if elapsed := time.Since(start); elapsed < 800*time.Millisecond {
		t.Fatalf("ReadInventoryProviders() took %v, want max_parallel=1 to serialize inventory fetches", elapsed)
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
