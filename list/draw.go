// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package list

import (
	"fmt"
	"strings"

	runewidth "github.com/mattn/go-runewidth"
	termbox "github.com/nsf/termbox-go"
)

// Draw a string based on the specified coordinate information
func drawLine(x, y int, str string, colorNum int, backColorNum int) {
	color := termbox.Attribute(colorNum + 1)
	backColor := termbox.Attribute(backColorNum + 1)

	// View Multi-Byte
	for _, char := range str {
		termbox.SetCell(x, y, char, color, backColor)
		x += runewidth.RuneWidth(char)
	}
}

// Highlight lines and draw text based on filtering results
func drawFilterLine(x, y int, str string, colorNum int, backColorNum int, keywordColorNum int, searchText string) {
	// SearchText Bounds Space
	searchWords := strings.Fields(searchText)

	for i := 0; i < len(searchWords); i += 1 {
		searchLowLine := strings.ToLower(str)
		searchKeyword := strings.ToLower(searchWords[i])
		searchKeywordLen := len(searchKeyword)
		searchKeywordCount := strings.Count(searchLowLine, searchKeyword)

		charLocation := 0
		for j := 0; j < searchKeywordCount; j += 1 {
			searchLineData := ""

			// Countermeasure "slice bounds out of range"
			if charLocation < len(str) {
				searchLineData = str[charLocation:]
			}
			searchLineDataStr := string(searchLineData)
			searchKeywordIndex := strings.Index(strings.ToLower(searchLineDataStr), searchKeyword)

			charLocation = charLocation + searchKeywordIndex
			keyword := ""

			// Countermeasure "slice bounds out of range"
			if charLocation < len(str) {
				keyword = str[charLocation : charLocation+searchKeywordLen]
			}

			// Get Multibyte Charctor Location
			multibyteStrCheckLine := str[:charLocation]
			multiByteCharLocation := 0
			for _, multiByteChar := range multibyteStrCheckLine {
				multiByteCharLocation += runewidth.RuneWidth(multiByteChar)
			}

			drawLine(x+multiByteCharLocation, y, keyword, keywordColorNum, backColorNum)
			charLocation = charLocation + searchKeywordLen
		}
	}
}

// draw list
func (l *ListInfo) draw() {
	l.Term.Headline = 2
	l.Term.LeftMargin = 2
	l.Term.Color = 255
	l.Term.BackgroundColor = 255

	termbox.Clear(termbox.Attribute(l.Term.Color+1), termbox.Attribute(l.Term.BackgroundColor+1))

	// Get Terminal Size
	_, height := termbox.Size()
	height = height - l.Term.Headline

	// Set View List Range
	firstLine := (l.CursorLine/height)*height + 1
	lastLine := firstLine + height

	var viewList []string
	if lastLine > len(l.ViewText) {
		viewList = l.ViewText[firstLine:]
	} else {
		viewList = l.ViewText[firstLine:lastLine]
	}
	cursor := l.CursorLine - firstLine + 1

	// View Head
	drawLine(0, 0, l.Prompt, 3, l.Term.BackgroundColor)
	drawLine(len(l.Prompt), 0, l.Keyword, l.Term.Color, l.Term.BackgroundColor)
	drawLine(l.Term.LeftMargin, 1, l.ViewText[0], 3, l.Term.BackgroundColor)

	// View List
	for listKey, listValue := range viewList {
		paddingData := fmt.Sprintf("%-1000s", listValue)
		// Set cursor color
		cursorColor := l.Term.Color
		cursorBackColor := l.Term.BackgroundColor
		keywordColor := 5

		for _, selectedLine := range l.SelectName {
			if strings.Split(listValue, " ")[0] == selectedLine {
				cursorColor = 0
				cursorBackColor = 6
			}
		}

		if listKey == cursor {
			// Select line color
			cursorColor = 0
			cursorBackColor = 2
		}

		// Draw filter line
		drawLine(l.Term.LeftMargin, listKey+l.Term.Headline, paddingData, cursorColor, cursorBackColor)

		// Keyword Highlight
		drawFilterLine(l.Term.LeftMargin, listKey+l.Term.Headline, paddingData, cursorColor, cursorBackColor, keywordColor, l.Keyword)
		listKey += 1
	}

	// Multi-Byte SetCursor
	x := 0
	for _, c := range l.Keyword {
		x += runewidth.RuneWidth(c)
	}
	termbox.SetCursor(len(l.Prompt)+x, 0)
	termbox.Flush()
}
