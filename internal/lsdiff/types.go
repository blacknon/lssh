// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsdiff

type Target struct {
	Host       string
	RemotePath string
	Title      string
}

type Document struct {
	Target Target
	Lines  []string
	Error  string
}

type LineCell struct {
	LineNo  int
	Text    string
	Present bool
}

type AlignedRow struct {
	Cells   []LineCell
	Changed bool
}

type Comparison struct {
	Documents []Document
	Rows      []AlignedRow
}
