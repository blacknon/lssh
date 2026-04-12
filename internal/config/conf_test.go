// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
)

func TestCheckFormatServerConf(t *testing.T) {
	type TestData struct {
		desc   string
		c      Config
		expect bool
	}
	tds := []TestData{
		{
			desc: "Address, user and password",
			c: Config{
				Server: map[string]ServerConfig{
					"a": {Addr: "192.168.100.101", User: "test", Pass: "Password"},
				},
			},
			expect: true,
		},
		{
			desc: "Empty address",
			c: Config{
				Server: map[string]ServerConfig{
					"b": {Addr: "", User: "test", Pass: "Password"},
				},
			},
			expect: false,
		},
		{
			desc: "Empty user",
			c: Config{
				Server: map[string]ServerConfig{
					"c": {Addr: "192.168.100.101", User: "", Pass: "Password"},
				},
			},
			expect: false,
		},
		{
			desc: "Empty password",
			c: Config{
				Server: map[string]ServerConfig{
					"d": {Addr: "192.168.100.101", User: "test", Pass: ""},
				},
			},
			expect: false,
		},
		{
			desc: "1 server config is illegal",
			c: Config{
				Server: map[string]ServerConfig{
					"a": {Addr: "192.168.100.101", User: "test", Pass: "Password"},
					"b": {Addr: "", User: "test", Pass: "Password"},
					"e": {Addr: "192.168.100.101", User: "test", Pass: "Password"},
				},
			},
			expect: false,
		},
	}
	for _, v := range tds {
		got := v.c.checkFormatServerConf()
		assert.Equal(t, v.expect, got, v.desc)
	}
}

func TestCheckFormatServerConfAuth(t *testing.T) {
	type TestData struct {
		desc   string
		c      ServerConfig
		expect bool
	}
	tds := []TestData{
		{desc: "Password", c: ServerConfig{Pass: "Password"}, expect: true},
		{desc: "Secret key file", c: ServerConfig{Key: "/tmp/key.pem"}, expect: true},
		{desc: "Cert file", c: ServerConfig{Cert: "/tmp/key.crt"}, expect: true},
		{desc: "Agent auth", c: ServerConfig{AgentAuth: true}, expect: true},
		// {desc: "File exists", c: ServerConfig{PKCS11Provider: "/path/to/providor"}, expect: true},
		{desc: "Key files", c: ServerConfig{Keys: []string{"/tmp/key.pem", "/tmp/key2.pem"}}, expect: true},
		{desc: "Passwords", c: ServerConfig{Passes: []string{"Pass1", "Pass2"}}, expect: true},
	}
	for _, v := range tds {
		got := checkFormatServerConfAuth(v.c)
		assert.Equal(t, v.expect, got, v.desc)
	}
}

func TestServerConfigReduct(t *testing.T) {
	type TestData struct {
		desc                   string
		perConfig, childConfig ServerConfig
		expect                 ServerConfig
	}
	tds := []TestData{
		{
			desc:        "Set perConfig addr to child config",
			perConfig:   ServerConfig{Addr: "192.168.100.101", User: "pertest"},
			childConfig: ServerConfig{User: "test"},
			expect:      ServerConfig{Addr: "192.168.100.101", User: "test"},
		},
		{
			desc:        "Child config is empty",
			perConfig:   ServerConfig{Addr: "192.168.100.101"},
			childConfig: ServerConfig{},
			expect:      ServerConfig{Addr: "192.168.100.101"},
		},
		{
			desc:        "Per config is empty",
			perConfig:   ServerConfig{},
			childConfig: ServerConfig{User: "test"},
			expect:      ServerConfig{User: "test"},
		},
		{
			desc:        "Both empty",
			perConfig:   ServerConfig{},
			childConfig: ServerConfig{},
			expect:      ServerConfig{},
		},
	}
	for _, v := range tds {
		got := serverConfigReduct(v.perConfig, v.childConfig)
		assert.Equal(t, v.expect, got, v.desc)
	}
}

func TestGetNameList(t *testing.T) {
	type TestData struct {
		desc     string
		listConf Config
		expect   []string
	}
	tds := []TestData{
		{
			desc: "",
			listConf: Config{
				Server: map[string]ServerConfig{
					"a": {},
					"b": {Ignore: true},
				},
			},
			expect: []string{"a"},
		},
		{
			desc: "",
			listConf: Config{
				Server: map[string]ServerConfig{},
			},
			expect: nil,
		},
	}
	for _, v := range tds {
		got := GetNameList(v.listConf)
		assert.Equal(t, v.expect, got, v.desc)
	}
}

func TestResolveConditionalMatchesPriorityAndOrder(t *testing.T) {
	originalDetector := detectMatchContext
	t.Cleanup(func() { detectMatchContext = originalDetector })
	detectMatchContext = func(reqs matchRequirements) matchContext {
		return matchContext{
			LocalIPs: []netip.Addr{netip.MustParseAddr("100.64.12.3")},
		}
	}

	var cfg Config
	_, err := toml.Decode(`
[server.test]
addr = "172.16.10.51"
user = "root"
pass = "secret"

[server.test.match.second]
priority = 1
when.local_ip_in = ["100.64.0.0/10"]
addr = "172.31.0.52"

[server.test.match.first]
priority = 1
when.local_ip_in = ["100.64.0.0/10"]
addr = "172.31.0.51"
`, &cfg)
	if !assert.NoError(t, err) {
		return
	}
	applyMatchMetadata(&cfg, mustDecodeMeta(t, `
[server.test]
addr = "172.16.10.51"
user = "root"
pass = "secret"

[server.test.match.second]
priority = 1
when.local_ip_in = ["100.64.0.0/10"]
addr = "172.31.0.52"

[server.test.match.first]
priority = 1
when.local_ip_in = ["100.64.0.0/10"]
addr = "172.31.0.51"
`))

	err = cfg.ResolveConditionalMatches()
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "172.31.0.51", cfg.Server["test"].Addr)
}

func TestDecodeConfigFileYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lssh.yaml")

	err := os.WriteFile(path, []byte(`
common:
  user: common-user
  pass: common-pass
server:
  demo:
    addr: 192.168.10.20
    user: demo-user
    pass: secret
    control_persist: 15s
    match:
      office:
        priority: 10
        when:
          os_in:
            - darwin
        proxy: bastion
      local:
        when:
          hostname_in:
            - mbp
        addr: 127.0.0.1
sshconfig:
  extra:
    path: ~/.ssh/config
    when:
      os_in:
        - darwin
`), 0o600)
	if !assert.NoError(t, err) {
		return
	}

	var cfg Config
	err = decodeConfigFile(path, &cfg)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "common-user", cfg.Common.User)
	assert.Equal(t, "demo-user", cfg.Server["demo"].User)
	assert.Equal(t, ControlPersistDuration(15), cfg.Server["demo"].ControlPersist)

	office := cfg.Server["demo"].Match["office"]
	assert.Equal(t, 10, office.EffectivePriority())
	assert.True(t, office.priorityDefined)
	assert.True(t, office.IsDefined("proxy"))
	assert.Equal(t, "bastion", office.Proxy)

	local := cfg.Server["demo"].Match["local"]
	assert.False(t, local.priorityDefined)
	assert.Equal(t, 100, local.EffectivePriority())
	assert.True(t, local.IsDefined("addr"))

	assert.Equal(t, "~/.ssh/config", cfg.SSHConfig["extra"].Path)
	assert.Equal(t, []string{"darwin"}, cfg.SSHConfig["extra"].When.OSIn)
}

func TestResolveConditionalMatchesMergeByPriority(t *testing.T) {
	cfg := Config{
		Server: map[string]ServerConfig{
			"test": {
				Addr: "10.0.0.10",
				User: "root",
				Pass: "secret",
				Match: map[string]ServerMatchConfig{
					"network": {
						Priority:        10,
						priorityDefined: true,
						order:           1,
						When: ServerMatchWhen{
							LocalIPIn: []string{"100.64.0.0/10"},
						},
						Proxy: "bastion",
						Note:  "network matched",
						definedKeys: map[string]bool{
							"proxy": true,
							"note":  true,
						},
					},
					"os": {
						Priority:        50,
						priorityDefined: true,
						order:           2,
						When: ServerMatchWhen{
							OSIn: []string{"darwin"},
						},
						User: "mac-user",
						definedKeys: map[string]bool{
							"user": true,
						},
					},
				},
			},
		},
	}

	originalDetector := detectMatchContext
	t.Cleanup(func() { detectMatchContext = originalDetector })
	detectMatchContext = func(reqs matchRequirements) matchContext {
		return matchContext{
			LocalIPs: []netip.Addr{netip.MustParseAddr("100.64.1.10")},
			OS:       "darwin",
		}
	}

	err := cfg.ResolveConditionalMatches()
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "bastion", cfg.Server["test"].Proxy)
	assert.Equal(t, "network matched", cfg.Server["test"].Note)
	assert.Equal(t, "mac-user", cfg.Server["test"].User)
}

func TestResolveConditionalMatchesExplicitFalseOverrides(t *testing.T) {
	cfg := Config{
		Server: map[string]ServerConfig{
			"test": {
				Addr:   "10.0.0.10",
				User:   "root",
				Pass:   "secret",
				Ignore: true,
				Match: map[string]ServerMatchConfig{
					"visible_on_mac": {
						Priority:        10,
						priorityDefined: true,
						order:           1,
						When: ServerMatchWhen{
							OSIn: []string{"darwin"},
						},
						Ignore: false,
						definedKeys: map[string]bool{
							"ignore": true,
						},
					},
				},
			},
		},
	}

	originalDetector := detectMatchContext
	t.Cleanup(func() { detectMatchContext = originalDetector })
	detectMatchContext = func(reqs matchRequirements) matchContext {
		return matchContext{OS: "darwin"}
	}

	err := cfg.ResolveConditionalMatches()
	if !assert.NoError(t, err) {
		return
	}

	assert.False(t, cfg.Server["test"].Ignore)
}

func TestResolveConditionalMatchesOSTermEnv(t *testing.T) {
	cfg := Config{
		Server: map[string]ServerConfig{
			"test": {
				Addr: "10.0.0.10",
				User: "root",
				Pass: "secret",
				Match: map[string]ServerMatchConfig{
					"mac_iterm": {
						Priority:        10,
						priorityDefined: true,
						order:           1,
						When: ServerMatchWhen{
							OSIn:   []string{"darwin"},
							TermIn: []string{"iterm2"},
							EnvIn:  []string{"SSH_AUTH_SOCK"},
							EnvValueIn: []string{
								"TERM_PROGRAM=iTerm.app",
							},
						},
						Note: "matched",
						definedKeys: map[string]bool{
							"note": true,
						},
					},
				},
			},
		},
	}

	originalDetector := detectMatchContext
	t.Cleanup(func() { detectMatchContext = originalDetector })
	detectMatchContext = func(reqs matchRequirements) matchContext {
		return matchContext{
			OS:       "darwin",
			Terms:    []string{"iterm2", "xterm-256color"},
			EnvNames: []string{"SSH_AUTH_SOCK", "HOME"},
			EnvValues: map[string]string{
				"TERM_PROGRAM":  "iTerm.app",
				"SSH_AUTH_SOCK": "/tmp/agent.sock",
			},
		}
	}

	err := cfg.ResolveConditionalMatches()
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "matched", cfg.Server["test"].Note)
}

func TestResolveConditionalMatchesEnvValueNotIn(t *testing.T) {
	cfg := Config{
		Server: map[string]ServerConfig{
			"test": {
				Addr: "10.0.0.10",
				User: "root",
				Pass: "secret",
				Match: map[string]ServerMatchConfig{
					"block_terminal_app": {
						Priority:        10,
						priorityDefined: true,
						order:           1,
						When: ServerMatchWhen{
							EnvValueNotIn: []string{"TERM_PROGRAM=Apple_Terminal"},
						},
						Note: "allowed",
						definedKeys: map[string]bool{
							"note": true,
						},
					},
				},
			},
		},
	}

	originalDetector := detectMatchContext
	t.Cleanup(func() { detectMatchContext = originalDetector })
	detectMatchContext = func(reqs matchRequirements) matchContext {
		return matchContext{
			EnvValues: map[string]string{
				"TERM_PROGRAM": "iTerm.app",
			},
		}
	}

	err := cfg.ResolveConditionalMatches()
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "allowed", cfg.Server["test"].Note)
}

func TestResolveConditionalMatchesLocalIPFallbackAndIgnore(t *testing.T) {
	originalDetector := detectMatchContext
	t.Cleanup(func() { detectMatchContext = originalDetector })

	cfg := Config{
		Server: map[string]ServerConfig{
			"sv-pve01": {
				Addr: "172.16.10.51",
				User: "root",
				Pass: "secret",
				Match: map[string]ServerMatchConfig{
					"from_vpn1": {
						When: ServerMatchWhen{
							LocalIPIn: []string{"100.64.0.0/10"},
						},
						Addr: "172.31.0.51",
					},
					"from_other_net": {
						Priority: 2,
						When: ServerMatchWhen{
							LocalIPNotIn: []string{"100.64.0.0/10", "172.16.100.0/24"},
						},
						Addr:   "172.31.3.51",
						Ignore: true,
					},
				},
			},
		},
	}
	cfg.Server["sv-pve01"].Match["from_vpn1"] = ServerMatchConfig{
		Priority: 1,
		When: ServerMatchWhen{
			LocalIPIn: []string{"100.64.0.0/10"},
		},
		Addr:            "172.31.0.51",
		definedKeys:     map[string]bool{"addr": true},
		priorityDefined: true,
		order:           1,
	}
	cfg.Server["sv-pve01"].Match["from_other_net"] = ServerMatchConfig{
		Priority: 2,
		When: ServerMatchWhen{
			LocalIPNotIn: []string{"100.64.0.0/10", "172.16.100.0/24"},
		},
		Addr:            "172.31.3.51",
		Ignore:          true,
		definedKeys:     map[string]bool{"addr": true, "ignore": true},
		priorityDefined: true,
		order:           2,
	}

	detectMatchContext = func(reqs matchRequirements) matchContext {
		return matchContext{LocalIPs: []netip.Addr{netip.MustParseAddr("192.168.1.50")}}
	}
	err := cfg.ResolveConditionalMatches()
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "172.31.3.51", cfg.Server["sv-pve01"].Addr)
	assert.True(t, cfg.Server["sv-pve01"].Ignore)
	assert.Empty(t, GetNameList(cfg))

	cfg.Server["sv-pve01"] = ServerConfig{
		Addr:  "172.16.10.51",
		User:  "root",
		Pass:  "secret",
		Match: cfg.Server["sv-pve01"].Match,
	}
	detectMatchContext = func(reqs matchRequirements) matchContext {
		return matchContext{LocalIPs: []netip.Addr{netip.MustParseAddr("172.16.100.20")}}
	}
	err = cfg.ResolveConditionalMatches()
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "172.16.10.51", cfg.Server["sv-pve01"].Addr)
	assert.False(t, cfg.Server["sv-pve01"].Ignore)
}

func TestResolveConditionalMatchesInvalidCIDR(t *testing.T) {
	cfg := Config{
		Server: map[string]ServerConfig{
			"example": {
				Addr: "10.0.0.1",
				User: "root",
				Pass: "secret",
				Match: map[string]ServerMatchConfig{
					"broken": {
						When: ServerMatchWhen{
							LocalIPIn: []string{"bad-cidr"},
						},
					},
				},
			},
		},
	}

	err := cfg.ResolveConditionalMatches()
	if !assert.Error(t, err) {
		return
	}
	assert.Contains(t, err.Error(), "invalid IP/CIDR")
}

func TestActiveOpenSSHConfigsFiltersByWhen(t *testing.T) {
	originalDetector := detectMatchContext
	t.Cleanup(func() { detectMatchContext = originalDetector })
	detectMatchContext = func(reqs matchRequirements) matchContext {
		return matchContext{
			LocalIPs: []netip.Addr{netip.MustParseAddr("172.31.0.10")},
			Username: "demo",
		}
	}

	cfg := Config{
		SSHConfig: map[string]OpenSSHConfig{
			"default": {
				Path: "~/.ssh/config",
			},
			"frontend": {
				Path: "~/.ssh/frontend",
				When: ServerMatchWhen{
					LocalIPIn: []string{"172.31.0.0/24"},
				},
			},
			"admin_only": {
				Path: "~/.ssh/admin",
				When: ServerMatchWhen{
					UsernameIn: []string{"admin"},
				},
			},
		},
	}

	got := cfg.activeOpenSSHConfigs()
	if len(got) != 2 {
		t.Fatalf("len(activeOpenSSHConfigs()) = %d, want 2", len(got))
	}

	paths := []string{got[0].Path, got[1].Path}
	assert.Contains(t, paths, "~/.ssh/config")
	assert.Contains(t, paths, "~/.ssh/frontend")
	assert.NotContains(t, paths, "~/.ssh/admin")
}

func TestValidateOpenSSHConfigWhensInvalidCIDR(t *testing.T) {
	cfg := Config{
		SSHConfig: map[string]OpenSSHConfig{
			"bad": {
				Path: "~/.ssh/config",
				When: ServerMatchWhen{
					LocalIPIn: []string{"bad-cidr"},
				},
			},
		},
	}

	_, err := cfg.validateOpenSSHConfigWhens()
	if !assert.Error(t, err) {
		return
	}
	assert.Contains(t, err.Error(), "invalid IP/CIDR")
}

func TestReadOpenSSHConfigSkipsMissingDefaultConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := Config{
		Common: ServerConfig{
			User: "demo",
			Pass: "secret",
		},
		Server: map[string]ServerConfig{
			"vm-mng": {
				Addr: "192.0.2.10",
				User: "admin",
				Pass: "secret",
			},
		},
	}

	cfg.ReadOpenSSHConfig()

	if got := cfg.Server["vm-mng"].User; got != "admin" {
		t.Fatalf("cfg.Server[vm-mng].User = %q, want %q", got, "admin")
	}
}

func mustDecodeMeta(t *testing.T, body string) toml.MetaData {
	t.Helper()

	var cfg Config
	md, err := toml.Decode(body, &cfg)
	if !assert.NoError(t, err) {
		return toml.MetaData{}
	}
	return md
}
