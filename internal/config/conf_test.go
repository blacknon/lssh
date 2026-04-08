// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

import (
	"net/netip"
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
	assert.Equal(t, "172.31.0.52", cfg.Server["test"].Addr)
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

func mustDecodeMeta(t *testing.T, body string) toml.MetaData {
	t.Helper()

	var cfg Config
	md, err := toml.Decode(body, &cfg)
	if !assert.NoError(t, err) {
		return toml.MetaData{}
	}
	return md
}
