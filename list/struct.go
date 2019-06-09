package list

import "github.com/blacknon/lssh/conf"

type ListInfo struct {
	Prompt     string      // prompt string
	NameList   []string    // name list
	SelectName []string    // selected name
	DataList   conf.Config // original config data list
	DataText   []string    // all data text
	ViewText   []string    // filtered text
	MultiFlag  bool        // multi select flag
	Keyword    string      // input keyword
	CursorLine int         // cursor line
	Term       TermInfo
}

type TermInfo struct {
	Headline        int
	LeftMargin      int
	Color           int
	BackgroundColor int
}

type ListArrayInfo struct {
	Name    string
	Connect string
	Note    string
}
