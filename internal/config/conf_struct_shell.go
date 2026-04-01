// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

// ShellConfig Structure for storing lssh-shell(parallel shell) settings.
type ShellConfig struct {
	// prompt
	Prompt  string `toml:"PROMPT"`  // lssh shell(parallel shell) prompt
	OPrompt string `toml:"OPROMPT"` // lssh shell(parallel shell) output prompt

	// message,title etc...
	Title string `toml:"title"`

	// history file
	HistoryFile string `toml:"histfile"`

	// pre | post command setting
	PreCmd  string `toml:"pre_cmd"`
	PostCmd string `toml:"post_cmd"`

	// alias
	Alias map[string]ShellAliasConfig `toml:"alias"`

	// outexec
	OutexecCmdConfigs map[string]ShellOutexecCmdConfig `toml:"outexecs"`
}

type ShellAliasConfig struct {
	// command
	Command string `toml:"command"`
}

// OutexecCmdConfig
type ShellOutexecCmdConfig struct {
	// path
	Path string `toml:"path"`
}
