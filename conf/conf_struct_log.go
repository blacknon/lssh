// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

// LogConfig store the contents about the terminal log.
// The log file name is created in "YYYYmmdd_HHMMSS_servername.log" of the specified directory.
type LogConfig struct {
	// Enable terminal logging.
	Enable bool `toml:"enable" yaml:"enable"`

	// Add a timestamp at the beginning of the terminal log line.
	Timestamp bool `toml:"timestamp" yaml:"timestamp"`

	// Specifies the directory for creating terminal logs.
	Dir string `toml:"dirpath" yaml:"dirpath"`

	// Logging with remove ANSI code.
	RemoveAnsiCode bool `toml:"remove_ansi_code" yaml:"remove_ansi_code"`
}
