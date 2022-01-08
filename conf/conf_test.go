// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

import (
	"testing"

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
					"a": ServerConfig{Addr: "192.168.100.101", User: "test", Pass: "Password"},
				},
			},
			expect: true,
		},
		{
			desc: "Empty address",
			c: Config{
				Server: map[string]ServerConfig{
					"b": ServerConfig{Addr: "", User: "test", Pass: "Password"},
				},
			},
			expect: false,
		},
		{
			desc: "Empty user",
			c: Config{
				Server: map[string]ServerConfig{
					"c": ServerConfig{Addr: "192.168.100.101", User: "", Pass: "Password"},
				},
			},
			expect: false,
		},
		{
			desc: "Empty password",
			c: Config{
				Server: map[string]ServerConfig{
					"d": ServerConfig{Addr: "192.168.100.101", User: "test", Pass: ""},
				},
			},
			expect: false,
		},
		{
			desc: "1 server config is illegal",
			c: Config{
				Server: map[string]ServerConfig{
					"a": ServerConfig{Addr: "192.168.100.101", User: "test", Pass: "Password"},
					"b": ServerConfig{Addr: "", User: "test", Pass: "Password"},
					"e": ServerConfig{Addr: "192.168.100.101", User: "test", Pass: "Password"},
				},
			},
			expect: false,
		},
	}
	for _, v := range tds {
		got := checkFormatServerConf(v.c)
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
					"a": ServerConfig{},
					"b": ServerConfig{},
				},
			},
			expect: []string{"a", "b"},
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
