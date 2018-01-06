package list

import (
	"fmt"
	"strings"

	runewidth "github.com/mattn/go-runewidth"
	termbox "github.com/nsf/termbox-go"
)

func drawLine(x, y int, str string, colorNum int, backColorNum int) {
	color := termbox.Attribute(colorNum + 1)
	backColor := termbox.Attribute(backColorNum + 1)
	// View Multi-Byte
	for _, char := range str {
		termbox.SetCell(x, y, char, color, backColor)
		x += runewidth.RuneWidth(char)
	}
}

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

// Draw List
func draw(serverNameList []string, lineData []string, selectCursor int, searchText string) {
	headLine := 2
	leftMargin := 2
	defaultColor := 255
	defaultBackColor := 255
	pronpt := "lssh>>"
	termbox.Clear(termbox.Attribute(defaultColor+1), termbox.Attribute(defaultBackColor+1))

	// Get Terminal Size
	_, height := termbox.Size()
	lineHeight := height - headLine

	// Set View List Range
	viewFirstLine := (selectCursor/lineHeight)*lineHeight + 1
	viewLastLine := viewFirstLine + lineHeight
	var serverViewList []string
	if viewLastLine > len(serverNameList) {
		serverViewList = serverNameList[viewFirstLine:]
	} else {
		serverViewList = serverNameList[viewFirstLine:viewLastLine]
	}
	selectViewCursor := selectCursor - viewFirstLine + 1

	// View Head
	drawLine(0, 0, pronpt, 3, defaultBackColor)
	drawLine(len(pronpt), 0, searchText, defaultColor, defaultBackColor)
	drawLine(leftMargin, 1, serverNameList[0], 3, defaultBackColor)

	// View List
	for listKey, listValue := range serverViewList {
		paddingListValue := fmt.Sprintf("%-1000s", listValue)
		// Set cursor color
		cursorColor := defaultColor
		cursorBackColor := defaultBackColor
		keywordColor := 5

		for _, selectedLine := range lineData {
			if strings.Split(listValue, " ")[0] == selectedLine {
				cursorColor = 0
				cursorBackColor = 6
			}
		}

		if listKey == selectViewCursor {
			// Select line color
			cursorColor = 0
			cursorBackColor = 2
		}

		// Draw filter line
		drawLine(leftMargin, listKey+headLine, paddingListValue, cursorColor, cursorBackColor)

		// Keyword Highlight
		drawFilterLine(leftMargin, listKey+headLine, paddingListValue, cursorColor, cursorBackColor, keywordColor, searchText)
		listKey += 1
	}

	// Multi-Byte SetCursor
	x := 0
	for _, c := range searchText {
		x += runewidth.RuneWidth(c)
	}
	termbox.SetCursor(len(pronpt)+x, 0)
	termbox.Flush()
}
