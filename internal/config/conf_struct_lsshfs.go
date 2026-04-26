// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

type LsshfsPlatformConfig struct {
	MountOptions []string `toml:"mount_options" yaml:"mount_options"`
}

type LsshfsConfig struct {
	MountOptions []string             `toml:"mount_options" yaml:"mount_options"`
	Darwin       LsshfsPlatformConfig `toml:"darwin" yaml:"darwin"`
	Linux        LsshfsPlatformConfig `toml:"linux" yaml:"linux"`
	Windows      LsshfsPlatformConfig `toml:"windows" yaml:"windows"`
}
