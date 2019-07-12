/*
list package creates a TUI list based on the contents specified in a structure, and returns the selected row.
*/
package list

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/tabwriter"

	termbox "github.com/nsf/termbox-go"
)

// TODO(blacknon):
//     - 外部のライブラリとして外出しする
//     - tomlやjsonなどを渡して、出力項目を指定できるようにする
//     - 指定した項目だけでの検索などができるようにする
//     - 検索方法の充実化(regexでの検索など)
//     - 内部でのウィンドウの実装
//         - 項目について、更新や閲覧ができるようにする
//     - キーバインドの設定変更

// arrayContains returns that arr contains str.
func arrayContains(arr []string, str string) bool {
	for _, v := range arr {
		if v == str {
			return true
		}
	}
	return false
}

// Toggle the selected state of cursor line.
func (l *ListInfo) toggle(newLine string) {
	tmpList := []string{}

	addFlag := true
	for _, selectedLine := range l.SelectName {
		if selectedLine != newLine {
			tmpList = append(tmpList, selectedLine)
		} else {
			addFlag = false
		}
	}
	if addFlag == true {
		tmpList = append(tmpList, newLine)
	}
	l.SelectName = []string{}
	l.SelectName = tmpList
}

// Toggle the selected state of the currently displayed list
func (l *ListInfo) allToggle(allFlag bool) {
	SelectedList := []string{}
	allSelectedList := []string{} // WARN: is not used
	// selectedList in allSelectedList
	for _, selectedLine := range l.SelectName {
		SelectedList = append(SelectedList, selectedLine)
	}

	// allFlag is False
	if allFlag == false {
		// On each lines that except a header line and are not selected line,
		// toggles left end fields
		for _, addLine := range l.ViewText[1:] {
			addName := strings.Fields(addLine)[0]
			if !arrayContains(SelectedList, addName) {
				allSelectedList = append(allSelectedList, addName)
				l.toggle(addName)
			}
		}
		return
	} else {
		// On each lines that except a header line, toggles left end fields
		for _, addLine := range l.ViewText[1:] {
			addName := strings.Fields(addLine)[0]
			l.toggle(addName)
		}
		return
	}
}

// Create view text (use text/tabwriter)
func (l *ListInfo) getText() {
	buffer := &bytes.Buffer{}
	tabWriterBuffer := new(tabwriter.Writer)
	tabWriterBuffer.Init(buffer, 0, 4, 8, ' ', 0)
	fmt.Fprintln(tabWriterBuffer, "ServerName \tConnect Information \tNote \t")

	// Create list table
	for _, key := range l.NameList {
		name := key
		conInfo := l.DataList.Server[key].User + "@" + l.DataList.Server[key].Addr
		note := l.DataList.Server[key].Note

		fmt.Fprintln(tabWriterBuffer, name+"\t"+conInfo+"\t"+note)
	}

	tabWriterBuffer.Flush()
	line, err := buffer.ReadString('\n')
	for err == nil {
		str := strings.Replace(line, "\t", " ", -1)
		l.DataText = append(l.DataText, str)
		line, err = buffer.ReadString('\n')
	}
}

// getFilterText updates l.ViewText with matching keyword (ignore case).
// DataText sets ViewText if keyword is empty.
func (l *ListInfo) getFilterText() {
	// Initialization ViewText
	l.ViewText = []string{}

	// SearchText Bounds Space
	keywords := strings.Fields(l.Keyword)
	r := l.DataText[1:]
	line := ""
	tmpText := []string{}
	l.ViewText = append(l.ViewText, l.DataText[0])

	// if No words
	if len(keywords) == 0 {
		l.ViewText = l.DataText
		return
	}

	for i := 0; i < len(keywords); i += 1 {
		lowKeyword := regexp.QuoteMeta(strings.ToLower(keywords[i]))
		re := regexp.MustCompile(lowKeyword)
		tmpText = []string{}

		for j := 0; j < len(r); j += 1 {
			line += string(r[j])
			if re.MatchString(strings.ToLower(line)) {
				tmpText = append(tmpText, line)
			}
			line = ""
		}
		r = tmpText
	}
	l.ViewText = append(l.ViewText, tmpText...)
	return
}

// View() display the list in TUI
func (l *ListInfo) View() {
	if err := termbox.Init(); err != nil {
		panic(err)
	}
	defer termbox.Close()

	// enable termbox mouse input
	termbox.SetInputMode(termbox.InputMouse)

	l.getText()
	l.keyEvent()
}
