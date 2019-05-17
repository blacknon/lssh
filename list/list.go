package list

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/tabwriter"

	termbox "github.com/nsf/termbox-go"
)

// arrayContains returns that arr contains str.
func arrayContains(arr []string, str string) bool {
	for _, v := range arr {
		if v == str {
			return true
		}
	}
	return false
}

// toggle select line (multi select)
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
	fmt.Fprintln(tabWriterBuffer, "ServerName \tConnect Infomation \tNote \t")

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
