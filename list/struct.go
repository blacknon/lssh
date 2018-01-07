package list

import "github.com/blacknon/lssh/conf"

type ListInfo struct {
	Prompt     string
	NameList   []string
	SelectName []string
	DataList   conf.Config
	ViewList   []string
	MultiFlag  bool
	Keyword    string
}

type ListArrayInfo struct {
	Name    string
	Connect string
	Note    string
}
