// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

// MonitorConfig stores monitor-specific configuration.
type MonitorConfig struct {
	Graph    MonitorGraphConfig    `toml:"graph" yaml:"graph"`
	GraphTab MonitorGraphTabConfig `toml:"graph_tab" yaml:"graph_tab"`
}

// MonitorGraphConfig stores graph scaling configuration for lsmon.
//
// Byte-rate settings are expressed in bytes per second. lsmon converts them to
// the current sampling interval internally, so users do not need to account for
// the monitor refresh period in config files.
type MonitorGraphConfig struct {
	FDMax uint64 `toml:"fd_max" yaml:"fd_max"`

	UseNetworkInterfaceSpeed  *bool             `toml:"use_network_interface_speed" yaml:"use_network_interface_speed"`
	NetworkDefaultBytesPerSec uint64            `toml:"network_default_bytes_per_sec" yaml:"network_default_bytes_per_sec"`
	NetworkBytesPerSec        map[string]uint64 `toml:"network_bytes_per_sec" yaml:"network_bytes_per_sec"`

	DiskDefaultReadBytesPerSec  uint64            `toml:"disk_default_read_bytes_per_sec" yaml:"disk_default_read_bytes_per_sec"`
	DiskDefaultWriteBytesPerSec uint64            `toml:"disk_default_write_bytes_per_sec" yaml:"disk_default_write_bytes_per_sec"`
	DiskReadBytesPerSec         map[string]uint64 `toml:"disk_read_bytes_per_sec" yaml:"disk_read_bytes_per_sec"`
	DiskWriteBytesPerSec        map[string]uint64 `toml:"disk_write_bytes_per_sec" yaml:"disk_write_bytes_per_sec"`
}

// MonitorGraphTabConfig stores optional metric graph tab layout for lsmon.
// When no panels are defined, the Graph tab is disabled.
type MonitorGraphTabConfig struct {
	Panels []MonitorGraphTabPanelConfig `toml:"panel" yaml:"panel"`
}

// MonitorGraphTabPanelConfig stores one graph pane in the Graph tab.
// Height is treated as a relative weight for vertical layout.
type MonitorGraphTabPanelConfig struct {
	Metric string `toml:"metric" yaml:"metric"`
	Title  string `toml:"title" yaml:"title"`
	Height int    `toml:"height" yaml:"height"`
}
