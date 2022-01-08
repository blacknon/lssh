// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsExist(t *testing.T) {
	type TestData struct {
		desc     string
		filename string
		expect   bool
	}
	tds := []TestData{
		{desc: "File exists", filename: "common.go", expect: true},
		{desc: "File doesn't exist", filename: "not_found_file", expect: false},
		{desc: "Empty path", filename: "", expect: false},
	}
	for _, v := range tds {
		got := IsExist(v.filename)
		assert.Equal(t, v.expect, got, v.desc)
	}
}

func TestMapReduce(t *testing.T) {
	type TestData struct {
		desc               string
		map1, map2, expect map[string]interface{}
	}
	tds := []TestData{
		{desc: "(string) Overrides value if key exists but value is empty", map1: map[string]interface{}{"a": "1", "b": "2", "c": "3"}, map2: map[string]interface{}{"a": "", "b": "1"}, expect: map[string]interface{}{"a": "1", "b": "1"}},
		{desc: "([]string) Overrides value if key exists but value is empty", map1: map[string]interface{}{"a": []string{"1"}, "b": "2", "c": "3"}, map2: map[string]interface{}{"a": "", "b": "1"}, expect: map[string]interface{}{"a": []string{"1"}, "b": "1"}},
		{desc: "(bool) Overrides value if key exists but value is empty", map1: map[string]interface{}{"a": true, "b": "2", "c": "3"}, map2: map[string]interface{}{"a": false, "b": "1"}, expect: map[string]interface{}{"a": true, "b": "1"}},

		{desc: "Returns map2 if map1 doesn't has keys", map1: map[string]interface{}{}, map2: map[string]interface{}{"a": "", "b": "1"}, expect: map[string]interface{}{"a": "", "b": "1"}},
	}
	for _, v := range tds {
		got := MapReduce(v.map1, v.map2)
		assert.Equal(t, v.expect, got, v.desc)
	}
}

func TestStructToMap(t *testing.T) {
	type A struct {
		S string
		I int
		f float32
	}
	type TestData struct {
		desc   string
		val    A
		mapVal map[string]interface{}
		ok     bool // is not used
	}
	tds := []TestData{
		{desc: "Converts struct to map", val: A{S: "a", I: 1}, mapVal: map[string]interface{}{"S": "a", "I": 1}, ok: false},
		{desc: "Sets zero value if val is a empty struct", val: A{}, mapVal: map[string]interface{}{"S": "", "I": 0}, ok: false},
		{desc: "Private field doesn't set", val: A{f: 1.0}, mapVal: map[string]interface{}{"S": "", "I": 0}, ok: false},
	}
	for _, v := range tds {
		val2 := v.val
		mapVal, ok := StructToMap(&val2)
		assert.Equal(t, v.mapVal, mapVal, v.desc)
		assert.Equal(t, v.ok, ok, v.desc)
	}
}

func TestMapToStruct(t *testing.T) {
	type A struct {
		S string
		I int
		f float32
	}
	type TestData struct {
		desc   string
		mapVal map[string]interface{}
		val    interface{}
		ok     bool // is not used
	}
	tds := []TestData{
		{desc: "Converts map to struct", mapVal: map[string]interface{}{"S": "a", "I": 1}, val: A{S: "a", I: 1}, ok: false},
		{desc: "Empty map", mapVal: map[string]interface{}{}, val: A{S: "", I: 0}, ok: false},
		// mapVal: map[string]interface{}{"f":1.0} raises panic
		// mapVal: map[string]interface{}{"NoField":1.0} raises panic
	}
	for _, v := range tds {
		val := A{}
		ok := MapToStruct(v.mapVal, &val)
		assert.Equal(t, v.val, val, v.desc)
		assert.Equal(t, v.ok, ok, v.desc)
	}
}

// TODO
// func TestGetFullPath(t *testing.T) {
// }

func TestGetMaxLength(t *testing.T) {
	type TestData struct {
		desc   string
		list   []string
		expect int
	}
	tds := []TestData{
		{desc: "list has 1 value", list: []string{"abc"}, expect: 3},
		{desc: "list has 2 value", list: []string{"abc", "abcde"}, expect: 5},
		{desc: "Multibyte", list: []string{"あいうえお"}, expect: 15},
		{desc: "list is empty", list: []string{}, expect: 0},
		{desc: "list is nil", list: nil, expect: 0},
	}
	for _, v := range tds {
		got := GetMaxLength(v.list)
		assert.Equal(t, v.expect, got, v.desc)
	}
}

// TODO
// func TestGetFilesBase64(t *testing.T) {
// }
