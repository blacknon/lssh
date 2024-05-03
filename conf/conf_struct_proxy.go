// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

// ProxyConfig is that stores Proxy server settings connected via http and socks5.
type ProxyConfig struct {
	Addr      string `toml:"addr"`
	Port      string `toml:"port"`
	User      string `toml:"user"`
	Pass      string `toml:"pass"`
	Proxy     string `toml:"proxy"`
	ProxyType string `toml:"proxy_type"`
	Note      string `toml:"note"`
}
