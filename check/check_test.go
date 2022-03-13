// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package check

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExistServer(t *testing.T) {
	type TestData struct {
		desc                  string
		inputServer, nameList []string
		expect                bool
	}
	tds := []TestData{
		{desc: "Server exists", inputServer: []string{"server1"}, nameList: []string{"server1", "server2"}, expect: true},
		{desc: "Server exists", inputServer: []string{"server2"}, nameList: []string{"server1", "server2"}, expect: true},
		{desc: "Server exists", inputServer: []string{"serverA", "server2"}, nameList: []string{"server1", "server2"}, expect: true},
		{desc: "Server doesn't exist", inputServer: []string{"server3"}, nameList: []string{"server1", "server2"}, expect: false},
		{desc: "Input servers are empty", inputServer: []string{}, nameList: []string{"server1", "server2"}, expect: false},
		{desc: "Input servers are nil", inputServer: nil, nameList: []string{"server1", "server2"}, expect: false},
		{desc: "Namelist are empty", inputServer: []string{"server1"}, nameList: []string{}, expect: false},
		{desc: "Namelist are nil", inputServer: []string{"server1"}, nameList: nil, expect: false},
		{desc: "Input servers and Namelist are empty", inputServer: []string{}, nameList: []string{}, expect: false},
		{desc: "Input servers and Namelist are nil", inputServer: nil, nameList: nil, expect: false},
	}
	for _, v := range tds {
		got := ExistServer(v.inputServer, v.nameList)
		assert.Equal(t, v.expect, got, v.desc)
	}
}

func TestParseScpPath(t *testing.T) {
	type TestData struct {
		desc     string
		arg      string
		isRemote bool
		path     string
	}
	tds := []TestData{
		{desc: "Local path (no :)", arg: "/tmp/a.txt", isRemote: false, path: "/tmp/a.txt"},
		{desc: "Local path (long)", arg: "local:/tmp/a.txt", isRemote: false, path: "/tmp/a.txt"},
		{desc: "Local path (short)", arg: "l:/tmp/a.txt", isRemote: false, path: "/tmp/a.txt"},
		{desc: "Remote path (long)", arg: "remote:/tmp/a.txt", isRemote: true, path: "/tmp/a.txt"},
		{desc: "Remote path (short)", arg: "r:/tmp/a.txt", isRemote: true, path: "/tmp/a.txt"},
		// run os.Exit(1) if arg is illegal path (ex: Z:/tmp/a.txt)
	}
	for _, v := range tds {
		isRemote, path := ParseScpPath(v.arg)
		assert.Equal(t, v.isRemote, isRemote, v.desc)
		assert.Equal(t, v.path, path, v.desc)
	}
}

func TestEscapePath(t *testing.T) {
	type TestData struct {
		desc   string
		str    string
		expect string
	}
	tds := []TestData{
		{desc: "No escape charcter", str: "hello", expect: "hello"},
		{desc: "Backslash", str: `a\b`, expect: `a\\b`},
		{desc: "Semicoron", str: `a;b`, expect: `a\;b`},
		{desc: "Whitespace", str: `a b`, expect: `a\ b`},
		{desc: "Empty string", str: ``, expect: ``},
	}
	for _, v := range tds {
		got := EscapePath(v.str)
		assert.Equal(t, v.expect, got, v.desc)
	}
}

func TestCheckTypeError(t *testing.T) {
	type TestData struct {
		desc       string
		r, l, toR  bool
		countHosts int
	}
	tds := []TestData{
		// exit 1 {desc: "", r: false, l: false, toR: false, countHosts: 0},
		{desc: "", r: false, l: false, toR: true, countHosts: 0},
		// exit 1 {desc: "", r: false, l: true, toR: false, countHosts: 0},
		{desc: "", r: false, l: true, toR: true, countHosts: 0},
		{desc: "", r: true, l: false, toR: false, countHosts: 0},
		{desc: "", r: true, l: false, toR: true, countHosts: 0},
		// exit 1 {desc: "", r: true, l: true, toR: false, countHosts: 0},
		// exit 1 {desc: "", r: true, l: true, toR: true, countHosts: 0},

		// exit 1 {desc: "", r: false, l: false, toR: false, countHosts: 1},
		{desc: "", r: false, l: false, toR: true, countHosts: 1},
		// exit 1 {desc: "", r: false, l: true, toR: false, countHosts: 1},
		{desc: "", r: false, l: true, toR: true, countHosts: 1},
		{desc: "", r: true, l: false, toR: false, countHosts: 1},
		// exit 1 {desc: "", r: true, l: false, toR: true, countHosts: 1},
		// exit 1 {desc: "", r: true, l: true, toR: false, countHosts: 1},
		// exit 1 {desc: "", r: true, l: true, toR: true, countHosts: 1},
	}
	for _, v := range tds {
		CheckTypeError(v.r, v.l, v.toR, v.countHosts)
	}
}
