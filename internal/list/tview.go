// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package list

import (
	"regexp"
	"sort"
	"strings"

	conf "github.com/blacknon/lssh/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TviewSelector is a tview-based selector that mirrors the classic list UI.
type TviewSelector struct {
	*tview.Flex

	app     *tview.Application
	list    *ListInfo
	prompt  *tview.TextView
	table   *tview.Table
	syncing bool

	done   func([]string)
	cancel func()
}

var (
	selectorNormalFG = tcell.ColorDefault
	selectorHeaderFG = tcell.ColorYellow
	selectorCursorBG = tcell.ColorGreen
	selectorCursorFG = tcell.ColorBlack
	selectorMarkedBG = tcell.ColorTeal
	selectorMarkedFG = tcell.ColorBlack
	selectorPromptFG = tcell.ColorYellow
	selectorMatchTag = "[#ff69b4]"
)

// NewTviewSelector creates a host selector widget.
func NewTviewSelector(app *tview.Application, prompt string, data conf.Config, names []string, multi bool) *TviewSelector {
	l := &ListInfo{
		Prompt:    prompt,
		NameList:  append([]string(nil), names...),
		DataList:  data,
		MultiFlag: multi,
	}
	l.getText()
	l.getFilterText()

	s := &TviewSelector{
		Flex: tview.NewFlex().SetDirection(tview.FlexRow),
		app:  app,
		list: l,
	}

	s.prompt = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false)
	s.prompt.SetBorder(false)

	s.table = tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)
	s.table.SetBorder(false)
	s.table.SetSelectedStyle(tcell.StyleDefault.
		Foreground(selectorCursorFG).
		Background(selectorCursorBG))
	s.table.SetInputCapture(s.handleInput)
	s.table.SetSelectionChangedFunc(func(row, _ int) {
		if s.syncing {
			return
		}
		if row <= 0 {
			s.list.CursorLine = 0
			s.render()
			return
		}
		cursor := row - 1
		if cursor >= 0 && cursor <= maxInt(len(s.list.ViewText)-2, 0) {
			s.list.CursorLine = cursor
			s.render()
		}
	})
	s.table.SetSelectedFunc(func(row, _ int) {
		_ = row
		s.confirm()
	})

	s.AddItem(s.prompt, 1, 0, false)
	s.AddItem(s.table, 0, 1, true)
	s.render()

	return s
}

func (s *TviewSelector) SetDoneFunc(fn func([]string)) {
	s.done = fn
}

func (s *TviewSelector) SetCancelFunc(fn func()) {
	s.cancel = fn
}

// FocusTarget returns the primitive that should receive keyboard input.
func (s *TviewSelector) FocusTarget() tview.Primitive {
	return s.table
}

func (s *TviewSelector) ListInfo() *ListInfo {
	return s.list
}

func (s *TviewSelector) Refresh() {
	s.render()
}

func (s *TviewSelector) handleInput(event *tcell.EventKey) *tcell.EventKey {
	headLine := 2
	height := s.pageHeight()
	allFlag := s.allSelectedVisible()

	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyCtrlC:
		s.fireCancel()
		return nil
	case tcell.KeyUp:
		if s.list.CursorLine > 0 {
			s.list.CursorLine--
		}
	case tcell.KeyDown:
		if s.list.CursorLine < len(s.list.ViewText)-headLine {
			s.list.CursorLine++
		}
	case tcell.KeyRight:
		nextPosition := ((s.list.CursorLine + height) / height) * height
		if nextPosition+2 <= len(s.list.ViewText) {
			s.list.CursorLine = nextPosition
		}
	case tcell.KeyLeft:
		beforePosition := ((s.list.CursorLine - height) / height) * height
		if beforePosition >= 0 {
			s.list.CursorLine = beforePosition
		}
	case tcell.KeyTab:
		if s.list.MultiFlag && len(s.list.ViewText) > s.list.CursorLine+1 {
			s.list.toggle(strings.Fields(s.list.ViewText[s.list.CursorLine+1])[0])
		}
		if s.list.CursorLine < len(s.list.ViewText)-headLine {
			s.list.CursorLine++
		}
	case tcell.KeyCtrlA:
		if s.list.MultiFlag {
			s.list.allToggle(allFlag)
		}
	case tcell.KeyEnter:
		s.confirm()
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(s.list.Keyword) > 0 {
			s.list.deleteRune()
			s.list.getFilterText()
			s.clampCursor()
		}
	case tcell.KeyRune:
		if event.Rune() == ' ' {
			s.list.Keyword += " "
			s.list.getFilterText()
			s.clampCursor()
			break
		}
		s.list.insertRune(event.Rune())
		s.list.getFilterText()
		s.clampCursor()
	default:
		return event
	}

	s.render()
	return nil
}

func (s *TviewSelector) render() {
	s.prompt.Clear()
	s.prompt.SetText("[yellow]" + s.list.Prompt + "[white]" + tview.Escape(s.list.Keyword))

	s.table.Clear()
	if len(s.list.ViewText) == 0 {
		return
	}

	header := s.list.ViewText[0]
	headerCell := tview.NewTableCell(header).
		SetSelectable(false).
		SetTextColor(selectorHeaderFG).
		SetSelectedStyle(tcell.StyleDefault.Foreground(selectorHeaderFG))
	s.table.SetCell(0, 0, headerCell)

	for i, line := range s.list.ViewText[1:] {
		cell := tview.NewTableCell(s.highlightLine(line))
		cell.SetExpansion(1)

		name := ""
		fields := strings.Fields(line)
		if len(fields) > 0 {
			name = fields[0]
		}

		if i == s.list.CursorLine {
			cell.SetTextColor(selectorCursorFG)
			cell.SetBackgroundColor(selectorCursorBG)
			cell.SetSelectedStyle(tcell.StyleDefault.
				Foreground(selectorCursorFG).
				Background(selectorCursorBG))
		} else if arrayContains(s.list.SelectName, name) {
			cell.SetTextColor(selectorMarkedFG)
			cell.SetBackgroundColor(selectorMarkedBG)
			cell.SetSelectedStyle(tcell.StyleDefault.
				Foreground(selectorMarkedFG).
				Background(selectorMarkedBG))
		} else {
			cell.SetTextColor(selectorNormalFG)
			cell.SetSelectedStyle(tcell.StyleDefault.
				Foreground(selectorCursorFG).
				Background(selectorCursorBG))
		}

		s.table.SetCell(i+1, 0, cell)
	}

	if len(s.list.ViewText) > 1 {
		s.syncing = true
		s.table.Select(s.list.CursorLine+1, 0)
		s.syncing = false
	}
}

func (s *TviewSelector) highlightLine(line string) string {
	keywords := strings.Fields(s.list.Keyword)
	if len(keywords) == 0 {
		return tview.Escape(line)
	}

	type span struct {
		start int
		end   int
	}

	var spans []span
	for _, keyword := range keywords {
		re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(keyword))
		if err != nil {
			continue
		}
		for _, loc := range re.FindAllStringIndex(line, -1) {
			if len(loc) != 2 || loc[0] == loc[1] {
				continue
			}
			spans = append(spans, span{start: loc[0], end: loc[1]})
		}
	}
	if len(spans) == 0 {
		return tview.Escape(line)
	}

	sort.Slice(spans, func(i, j int) bool {
		if spans[i].start == spans[j].start {
			return spans[i].end < spans[j].end
		}
		return spans[i].start < spans[j].start
	})

	merged := make([]span, 0, len(spans))
	for _, current := range spans {
		if len(merged) == 0 {
			merged = append(merged, current)
			continue
		}
		last := &merged[len(merged)-1]
		if current.start <= last.end {
			if current.end > last.end {
				last.end = current.end
			}
			continue
		}
		merged = append(merged, current)
	}

	var builder strings.Builder
	cursor := 0
	for _, item := range merged {
		if item.start > cursor {
			builder.WriteString(tview.Escape(line[cursor:item.start]))
		}
		builder.WriteString(selectorMatchTag)
		builder.WriteString(tview.Escape(line[item.start:item.end]))
		builder.WriteString("[-]")
		cursor = item.end
	}
	if cursor < len(line) {
		builder.WriteString(tview.Escape(line[cursor:]))
	}

	return builder.String()
}

func (s *TviewSelector) confirm() {
	if len(s.list.ViewText) <= s.list.CursorLine+1 {
		return
	}
	if len(s.list.SelectName) == 0 {
		s.list.SelectName = append(s.list.SelectName, strings.Fields(s.list.ViewText[s.list.CursorLine+1])[0])
	}
	if s.done != nil {
		s.done(append([]string(nil), s.list.SelectName...))
	}
}

func (s *TviewSelector) fireCancel() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *TviewSelector) clampCursor() {
	maxCursor := len(s.list.ViewText) - 2
	if maxCursor < 0 {
		maxCursor = 0
	}
	if s.list.CursorLine > maxCursor {
		s.list.CursorLine = maxCursor
	}
	if s.list.CursorLine < 0 {
		s.list.CursorLine = 0
	}
}

func (s *TviewSelector) pageHeight() int {
	_, _, _, h := s.table.GetInnerRect()
	if h <= 0 {
		return 1
	}
	return h
}

func (s *TviewSelector) allSelectedVisible() bool {
	if len(s.list.ViewText) <= 1 {
		return false
	}
	for _, line := range s.list.ViewText[1:] {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if !arrayContains(s.list.SelectName, fields[0]) {
			return false
		}
	}
	return true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
