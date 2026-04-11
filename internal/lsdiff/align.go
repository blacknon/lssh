// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsdiff

import "github.com/pmezard/go-difflib/difflib"

type pairAlignment struct {
	Mapped  []LineCell
	Inserts [][]LineCell
}

func AlignDocuments(documents []Document) Comparison {
	if len(documents) == 0 {
		return Comparison{}
	}

	base := documents[0]
	baseRows := make([]AlignedRow, len(base.Lines))
	for i, line := range base.Lines {
		baseRows[i] = AlignedRow{
			Cells: []LineCell{{
				LineNo:  i + 1,
				Text:    line,
				Present: true,
			}},
		}
	}

	insertSegments := make([][]AlignedRow, len(base.Lines)+1)
	docCount := 1

	for _, document := range documents[1:] {
		alignment := alignToBase(base.Lines, document.Lines)

		for segmentIndex := range insertSegments {
			insertSegments[segmentIndex] = mergeInsertedRows(insertSegments[segmentIndex], alignment.Inserts[segmentIndex], docCount)
		}

		for rowIndex := range baseRows {
			baseRows[rowIndex].Cells = append(baseRows[rowIndex].Cells, alignment.Mapped[rowIndex])
		}

		docCount++
	}

	rows := make([]AlignedRow, 0, len(baseRows))
	for i := 0; i < len(baseRows); i++ {
		rows = append(rows, insertSegments[i]...)
		rows = append(rows, baseRows[i])
	}
	rows = append(rows, insertSegments[len(insertSegments)-1]...)

	for i := range rows {
		rows[i].Changed = rowChanged(rows[i])
	}

	return Comparison{
		Documents: documents,
		Rows:      rows,
	}
}

func alignToBase(baseLines, otherLines []string) pairAlignment {
	matcher := difflib.NewMatcher(baseLines, otherLines)
	opcodes := matcher.GetOpCodes()

	mapped := make([]LineCell, len(baseLines))
	inserts := make([][]LineCell, len(baseLines)+1)

	for _, opcode := range opcodes {
		switch opcode.Tag {
		case 'e':
			for k := 0; k < opcode.I2-opcode.I1; k++ {
				mapped[opcode.I1+k] = LineCell{
					LineNo:  opcode.J1 + k + 1,
					Text:    otherLines[opcode.J1+k],
					Present: true,
				}
			}
		case 'd':
			for i := opcode.I1; i < opcode.I2; i++ {
				mapped[i] = LineCell{}
			}
		case 'i':
			for j := opcode.J1; j < opcode.J2; j++ {
				inserts[opcode.I1] = append(inserts[opcode.I1], LineCell{
					LineNo:  j + 1,
					Text:    otherLines[j],
					Present: true,
				})
			}
		case 'r':
			aLen := opcode.I2 - opcode.I1
			bLen := opcode.J2 - opcode.J1
			common := minInt(aLen, bLen)

			for k := 0; k < common; k++ {
				mapped[opcode.I1+k] = LineCell{
					LineNo:  opcode.J1 + k + 1,
					Text:    otherLines[opcode.J1+k],
					Present: true,
				}
			}

			for k := common; k < aLen; k++ {
				mapped[opcode.I1+k] = LineCell{}
			}

			for k := common; k < bLen; k++ {
				inserts[opcode.I1+common] = append(inserts[opcode.I1+common], LineCell{
					LineNo:  opcode.J1 + k + 1,
					Text:    otherLines[opcode.J1+k],
					Present: true,
				})
			}
		}
	}

	return pairAlignment{
		Mapped:  mapped,
		Inserts: inserts,
	}
}

func mergeInsertedRows(existing []AlignedRow, inserted []LineCell, previousDocCount int) []AlignedRow {
	maxRows := maxInt(len(existing), len(inserted))
	if maxRows == 0 {
		return nil
	}

	rows := make([]AlignedRow, 0, maxRows)
	for i := 0; i < maxRows; i++ {
		if i < len(existing) {
			row := existing[i]
			if i < len(inserted) {
				row.Cells = append(row.Cells, inserted[i])
			} else {
				row.Cells = append(row.Cells, LineCell{})
			}
			rows = append(rows, row)
			continue
		}

		cells := make([]LineCell, previousDocCount)
		cells = append(cells, inserted[i])
		rows = append(rows, AlignedRow{Cells: cells})
	}

	return rows
}

func rowChanged(row AlignedRow) bool {
	var reference string
	haveReference := false
	for _, cell := range row.Cells {
		if !cell.Present {
			if haveReference {
				return true
			}
			continue
		}

		if !haveReference {
			reference = cell.Text
			haveReference = true
			continue
		}

		if cell.Text != reference {
			return true
		}
	}

	if !haveReference {
		return false
	}

	for _, cell := range row.Cells {
		if !cell.Present {
			return true
		}
	}

	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
