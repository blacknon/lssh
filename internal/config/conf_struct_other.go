// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

// IncludeConfig specify the configuration file to include (ServerConfig only).
type IncludeConfig struct {
	Path string `toml:"path"`
}

// IncludesConfig specify the configuration file to include (ServerConfig only).
// Struct that can specify multiple files in array.
// TODO: ワイルドカード指定可能にする
type IncludesConfig struct {
	// example:
	// 	path = [
	// 		 "~/.lssh.d/home.conf"
	// 		,"~/.lssh.d/cloud.conf"
	// 	]
	Path []string `toml:"path"`
}

// OpenSSHConfig is read OpenSSH configuration file.
type OpenSSHConfig struct {
	Path    string `toml:"path"` // This is preferred
	Command string `toml:"command"`
	ServerConfig
}
