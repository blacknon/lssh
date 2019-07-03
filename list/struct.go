package list

import "github.com/blacknon/lssh/conf"

type ListInfo struct {
	// Incremental search line prompt string
	Prompt string

	NameList   []string
	SelectName []string
	DataList   conf.Config // original config data(struct)
	DataText   []string    // all data text list
	ViewText   []string    // filtered text list
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
