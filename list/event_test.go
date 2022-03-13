// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package list

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInsertRune(t *testing.T) {
	type TestData struct {
		desc      string
		l         ListInfo
		inputRune rune
		expect    string
	}
	tds := []TestData{
		{desc: "Input rune is a alphabet", l: ListInfo{Keyword: "a"}, inputRune: 'b', expect: "ab"},
		{desc: "Input rune is a multibyte character", l: ListInfo{Keyword: "a"}, inputRune: 'あ', expect: "aあ"},
	}
	for _, v := range tds {
		v.l.insertRune(v.inputRune)
		assert.Equal(t, v.expect, v.l.Keyword, v.desc)
	}
}

func TestDeleteRune(t *testing.T) {
	type TestData struct {
		desc   string
		l      ListInfo
		expect string
	}
	tds := []TestData{
		{desc: "Delete alphabet rune", l: ListInfo{Keyword: "abc"}, expect: "ab"},
		{desc: "Delete multibyte rune", l: ListInfo{Keyword: "あいう"}, expect: "あい"},
		{desc: "Expect is empty", l: ListInfo{Keyword: "a"}, expect: ""},
		// FIXME raise panic {desc: "Delete empty", l: ListInfo{Keyword: ""}, expect: ""},
	}
	for _, v := range tds {
		v.l.deleteRune()
		assert.Equal(t, v.expect, v.l.Keyword, v.desc)
	}
}
