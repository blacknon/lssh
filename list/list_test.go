// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package list

import (
	"testing"

	"github.com/blacknon/lssh/conf"
	"github.com/stretchr/testify/assert"
)

func TestArrayContains(t *testing.T) {
	type TestData struct {
		desc   string
		arr    []string
		str    string
		expect bool
	}
	tds := []TestData{
		{desc: "Contains word", arr: []string{"あ", "い"}, str: "あ", expect: true},
		{desc: "Contains word", arr: []string{"あ", "い"}, str: "い", expect: true},
		{desc: "Not contains word", arr: []string{"a", "b"}, str: "c", expect: false},
		{desc: "Not contains word", arr: []string{"a", "bb"}, str: "b", expect: false},
		{desc: "arr is empty", arr: []string{}, str: "c", expect: false},
		{desc: "arr is nil", arr: nil, str: "c", expect: false},
		{desc: "str is empty", arr: []string{"a", "b"}, str: "", expect: false},
	}
	for _, v := range tds {
		got := arrayContains(v.arr, v.str)
		assert.Equal(t, v.expect, got, v.desc)
	}
}

func TestToggle(t *testing.T) {
	type TestData struct {
		desc    string
		l       ListInfo
		newLine string
		expect  []string
	}
	tds := []TestData{
		{desc: "Existing word", l: ListInfo{SelectName: []string{"a", "b"}}, newLine: "a", expect: []string{"b"}},
		{desc: "Not existing word", l: ListInfo{SelectName: []string{"b"}}, newLine: "a", expect: []string{"b", "a"}},
		{desc: "Duplicated word", l: ListInfo{SelectName: []string{"a", "a"}}, newLine: "a", expect: []string{}},
		{desc: "SelectName is empty", l: ListInfo{SelectName: []string{}}, newLine: "a", expect: []string{"a"}},
		{desc: "SelectName is nil", l: ListInfo{SelectName: nil}, newLine: "a", expect: []string{"a"}},
	}
	for _, v := range tds {
		v.l.toggle(v.newLine)
		assert.Equal(t, v.expect, v.l.SelectName, v.desc)
	}
}

func TestAllToggle(t *testing.T) {
	texts := []string{
		"ServerName  ConnectInformation    Note",
		"prd_web1    user1@192.168.100.1   WebServer",
		"prd_web2    user1@192.168.100.2   WebServer",
		"prd_app1    user1@192.168.100.33  ApplicationServer",
		"prd_app2    user1@192.168.100.34  ApplicationServer",
		"prd_db1     user1@192.168.100.65  DatabaseServer",
		"dev_web1    user1@192.168.101.1   WebServer",
		"dev_web2    user1@192.168.101.2   WebServer",
		"dev_app1    user1@192.168.101.33  ApplicationServer",
		"dev_app2    user1@192.168.101.34  ApplicationServer",
		"dev_db1     user1@192.168.101.65  DatabaseServer",
	}

	type TestData struct {
		desc    string
		l       ListInfo
		allFlag bool
		expect  []string
	}
	tds := []TestData{
		{
			desc: "Toggle all",
			l: ListInfo{
				SelectName: []string{"dev_web1"},
				ViewText:   texts,
			},
			allFlag: true,
			expect:  []string{"prd_web1", "prd_web2", "prd_app1", "prd_app2", "prd_db1", "dev_web2", "dev_app1", "dev_app2", "dev_db1"},
		},
		{
			desc: "Toggle all",
			l: ListInfo{
				SelectName: []string{"prd_web1", "prd_web2", "prd_app1", "prd_app2", "prd_db1", "dev_web2", "dev_app1", "dev_app2", "dev_db1"},
				ViewText:   texts,
			},
			allFlag: true,
			expect:  []string{"dev_web1"},
		},
		{
			desc: "Select 1",
			l: ListInfo{
				SelectName: []string{"dev_web1"},
				ViewText:   texts,
			},
			allFlag: false,
			expect:  []string{"dev_web1", "prd_web1", "prd_web2", "prd_app1", "prd_app2", "prd_db1", "dev_web2", "dev_app1", "dev_app2", "dev_db1"},
		},
	}
	for _, v := range tds {
		v.l.allToggle(v.allFlag)
		assert.Equal(t, v.expect, v.l.SelectName, v.desc)
	}
}

func TestGetText(t *testing.T) {
	type TestData struct {
		desc   string
		l      ListInfo
		expect []string
	}
	tds := []TestData{
		{
			desc: "Get 1 server",
			l: ListInfo{
				NameList: []string{"dev_web1"},
				DataList: conf.Config{
					Server: map[string]conf.ServerConfig{
						"dev_web1": {User: "user1", Addr: "192.168.101.1", Note: "WebServer"},
						"dev_web2": {User: "user1", Addr: "192.168.101.2", Note: "WebServer"},
					},
				},
			},
			expect: []string{
				"ServerName         Connect Information        Note         \n",
				"dev_web1           user1@192.168.101.1        WebServer\n",
			},
		},
		{
			desc: "Get 2 server",
			l: ListInfo{
				NameList: []string{"dev_web1", "dev_web2"},
				DataList: conf.Config{
					Server: map[string]conf.ServerConfig{
						"dev_web1": {User: "user1", Addr: "192.168.101.1", Note: "WebServer"},
						"dev_web2": {User: "user1", Addr: "192.168.101.2", Note: "WebServer"},
					},
				},
			},
			expect: []string{
				"ServerName         Connect Information        Note         \n",
				"dev_web1           user1@192.168.101.1        WebServer\n",
				"dev_web2           user1@192.168.101.2        WebServer\n",
			},
		},
		{
			desc: "No NameList",
			l: ListInfo{
				NameList: []string{},
				DataList: conf.Config{
					Server: map[string]conf.ServerConfig{
						"dev_web1": {User: "user1", Addr: "192.168.101.1", Note: "WebServer"},
						"dev_web2": {User: "user1", Addr: "192.168.101.2", Note: "WebServer"},
					},
				},
			},
			expect: []string{
				"ServerName         Connect Information        Note         \n",
			},
		},
		{
			desc: "NameList is nil",
			l: ListInfo{
				NameList: nil,
				DataList: conf.Config{
					Server: map[string]conf.ServerConfig{
						"dev_web1": {User: "user1", Addr: "192.168.101.1", Note: "WebServer"},
						"dev_web2": {User: "user1", Addr: "192.168.101.2", Note: "WebServer"},
					},
				},
			},
			expect: []string{
				"ServerName         Connect Information        Note         \n",
			},
		},
	}
	for _, v := range tds {
		v.l.getText()
		assert.Equal(t, v.expect, v.l.DataText, v.desc)
	}
}

func TestGetFilterText(t *testing.T) {
	type TestData struct {
		desc   string
		l      ListInfo
		expect []string
	}
	tds := []TestData{
		{
			desc: "Expect is DataText if keyword is empty",
			l: ListInfo{
				Keyword: "",
				DataText: []string{
					"ServerName         Connect Information        Note",
					"dev_web1           user1@192.168.101.1        WebServer",
				},
			},
			expect: []string{
				"ServerName         Connect Information        Note",
				"dev_web1           user1@192.168.101.1        WebServer",
			},
		},
		{
			desc: "ServerName (text)",
			l: ListInfo{
				Keyword: "dev_web",
				DataText: []string{
					"ServerName         Connect Information        Note",
					"dev_web1           user1@192.168.101.1        WebServer",
					"dev_web2           user1@192.168.101.1        WebServer",
					"dev_app1           user1@192.168.101.1        ApplicationServer",
				},
			},
			expect: []string{
				"ServerName         Connect Information        Note",
				"dev_web1           user1@192.168.101.1        WebServer",
				"dev_web2           user1@192.168.101.1        WebServer",
			},
		},
		{
			desc: "Connect Information",
			l: ListInfo{
				Keyword: "33",
				DataText: []string{
					"ServerName         Connect Information        Note",
					"dev_web1           user1@192.168.101.1        WebServer",
					"dev_web2           user1@192.168.101.2        WebServer",
					"dev_app1           user1@192.168.101.33       ApplicationServer",
				},
			},
			expect: []string{
				"ServerName         Connect Information        Note",
				"dev_app1           user1@192.168.101.33       ApplicationServer",
			},
		},
		{
			desc: "Note (ignore case)",
			l: ListInfo{
				Keyword: "webserver",
				DataText: []string{
					"ServerName         Connect Information        Note",
					"dev_web1           user1@192.168.101.1        WebServer",
					"dev_web2           user1@192.168.101.2        WebServer",
					"dev_app1           user1@192.168.101.33       ApplicationServer",
				},
			},
			expect: []string{
				"ServerName         Connect Information        Note",
				"dev_web1           user1@192.168.101.1        WebServer",
				"dev_web2           user1@192.168.101.2        WebServer",
			},
		},
		// { // Can't use regexp
		// 	desc: "Regexp \\d",
		// 	l: ListInfo{
		// 		Keyword: `dev_web\d+`,
		// 		DataText: []string{
		// 			"ServerName         Connect Information        Note",
		// 			"dev_web            user1@192.168.101.99       WebServer",
		// 			"dev_web1           user1@192.168.101.1        WebServer",
		// 			"dev_web2           user1@192.168.101.2        WebServer",
		// 			"dev_webX           user1@192.168.101.31       WebServer",
		// 		},
		// 	},
		// 	expect: []string{
		// 		"ServerName         Connect Information        Note",
		// 		"dev_web1           user1@192.168.101.1        WebServer",
		// 		"dev_web2           user1@192.168.101.2        WebServer",
		// 	},
		// },
	}
	for _, v := range tds {
		v.l.getFilterText()
		assert.Equal(t, v.expect, v.l.ViewText, v.desc)
	}
}
