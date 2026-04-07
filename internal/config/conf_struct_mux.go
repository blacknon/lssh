// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

// MuxConfig stores lsmux key bindings.
type MuxConfig struct {
	Prefix               string `toml:"prefix"`
	Quit                 string `toml:"quit"`
	NewPage              string `toml:"new_page"`
	NewPane              string `toml:"new_pane"`
	SplitHorizontal      string `toml:"split_horizontal"`
	SplitVertical        string `toml:"split_vertical"`
	NextPane             string `toml:"next_pane"`
	NextPage             string `toml:"next_page"`
	PrevPage             string `toml:"prev_page"`
	PageList             string `toml:"page_list"`
	ClosePane            string `toml:"close_pane"`
	Broadcast            string `toml:"broadcast"`
	Transfer             string `toml:"transfer"`
	FocusBorderColor     string `toml:"focus_border_color"`
	FocusTitleColor      string `toml:"focus_title_color"`
	BroadcastBorderColor string `toml:"broadcast_border_color"`
	BroadcastTitleColor  string `toml:"broadcast_title_color"`
	DoneBorderColor      string `toml:"done_border_color"`
	DoneTitleColor       string `toml:"done_title_color"`
}

// ApplyDefaults fills empty key bindings with tmux-like defaults.
func (m MuxConfig) ApplyDefaults() MuxConfig {
	if m.Prefix == "" {
		m.Prefix = "Ctrl+A"
	}
	if m.Quit == "" {
		m.Quit = "&"
	}
	if m.NewPage == "" {
		m.NewPage = "c"
	}
	if m.NewPane == "" {
		m.NewPane = "s"
	}
	if m.SplitHorizontal == "" {
		m.SplitHorizontal = "\""
	}
	if m.SplitVertical == "" {
		m.SplitVertical = "%"
	}
	if m.NextPane == "" {
		m.NextPane = "o"
	}
	if m.NextPage == "" {
		m.NextPage = "n"
	}
	if m.PrevPage == "" {
		m.PrevPage = "p"
	}
	if m.PageList == "" {
		m.PageList = "w"
	}
	if m.ClosePane == "" {
		m.ClosePane = "x"
	}
	if m.Broadcast == "" {
		m.Broadcast = "b"
	}
	if m.Transfer == "" {
		m.Transfer = "f"
	}
	if m.FocusBorderColor == "" {
		m.FocusBorderColor = "green"
	}
	if m.FocusTitleColor == "" {
		m.FocusTitleColor = "green"
	}
	if m.BroadcastBorderColor == "" {
		m.BroadcastBorderColor = "yellow"
	}
	if m.BroadcastTitleColor == "" {
		m.BroadcastTitleColor = "yellow"
	}
	if m.DoneBorderColor == "" {
		m.DoneBorderColor = "gray"
	}
	if m.DoneTitleColor == "" {
		m.DoneTitleColor = "gray"
	}
	return m
}
