// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

// ShellConfig Structure for storing lssh-shell(parallel shell) settings.
type ShellConfig struct {
	// prompt
	Prompt  string `toml:"PROMPT" yaml:"PROMPT"`   // lssh shell(parallel shell) prompt
	OPrompt string `toml:"OPROMPT" yaml:"OPROMPT"` // lssh shell(parallel shell) output prompt

	// message,title etc...
	Title string `toml:"title" yaml:"title"`

	// history file
	HistoryFile string `toml:"histfile" yaml:"histfile"`

	// pre | post command setting
	PreCmd  string `toml:"pre_cmd" yaml:"pre_cmd"`
	PostCmd string `toml:"post_cmd" yaml:"post_cmd"`

	// alias
	Alias map[string]ShellAliasConfig `toml:"alias" yaml:"alias"`

	// outexec
	OutexecCmdConfigs map[string]ShellOutexecCmdConfig `toml:"outexecs" yaml:"outexecs"`
}

type ShellAliasConfig struct {
	// command
	Command string `toml:"command" yaml:"command"`
}

// OutexecCmdConfig
type ShellOutexecCmdConfig struct {
	// path
	Path string `toml:"path" yaml:"path"`
}
