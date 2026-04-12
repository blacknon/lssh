// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsdiff

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Viewer struct {
	app         *tview.Application
	comparison  Comparison
	layout      *tview.Flex
	panes       []*tview.TextView
	status      *tview.TextView
	searchInput *tview.InputField
	root        *tview.Flex
	rowOffset   int
	colOffset   int
	searchTerm  string
	layoutCols  int
	layoutWidth int
}

const minPaneWidth = 48

func NewViewer(comparison Comparison) *Viewer {
	v := &Viewer{
		app:        tview.NewApplication(),
		comparison: comparison,
		layout:     tview.NewFlex().SetDirection(tview.FlexRow),
		status:     tview.NewTextView().SetDynamicColors(true).SetWrap(false),
		searchInput: tview.NewInputField().
			SetLabel("/").
			SetFieldWidth(0),
	}

	for _, document := range comparison.Documents {
		textView := tview.NewTextView().
			SetDynamicColors(true).
			SetWrap(false).
			SetScrollable(true)
		textView.SetBorder(true)
		textView.SetTitle(" " + document.Target.Title + " ")
		if document.Error != "" {
			textView.SetTitle(" " + document.Target.Title + " [failed] ")
		}
		textView.SetInputCapture(v.handleInput)
		textView.SetMouseCapture(v.handleMouse)
		v.panes = append(v.panes, textView)
	}

	v.searchInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			v.searchTerm = strings.TrimSpace(v.searchInput.GetText())
			v.searchInput.SetText("")
			v.root.ResizeItem(v.searchInput, 0, 0)
			v.jumpToFirstMatch()
			v.render()
			v.app.SetFocus(v.root)
		case tcell.KeyEsc:
			v.searchInput.SetText("")
			v.root.ResizeItem(v.searchInput, 0, 0)
			v.app.SetFocus(v.root)
		}
	})
	v.searchInput.SetInputCapture(v.handleInput)

	v.root = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(v.layout, 0, 1, true).
		AddItem(v.status, 1, 0, false).
		AddItem(v.searchInput, 0, 0, false)

	v.layout.SetInputCapture(v.handleInput)
	v.root.SetInputCapture(v.handleInput)
	v.app.SetInputCapture(v.handleInput)
	v.app.EnableMouse(true)
	v.app.SetRoot(v.root, true)
	if len(v.panes) > 0 {
		v.app.SetFocus(v.panes[0])
	} else {
		v.app.SetFocus(v.root)
	}
	v.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		width, _ := screen.Size()
		v.rebuildLayout(width)
		v.render()
		return false
	})
	return v
}

func (v *Viewer) handleMouse(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
	if event == nil {
		return action, event
	}

	switch action {
	case tview.MouseScrollUp:
		v.scrollVertical(-1)
		go v.app.QueueUpdateDraw(func() {
			v.render()
		})
		return action, nil
	case tview.MouseScrollDown:
		v.scrollVertical(1)
		go v.app.QueueUpdateDraw(func() {
			v.render()
		})
		return action, nil
	default:
		return action, event
	}
}

func (v *Viewer) Run() error {
	return v.app.Run()
}

func (v *Viewer) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyCtrlQ:
		v.app.Stop()
		return nil
	case tcell.KeyUp:
		v.scrollVertical(-1)
		return nil
	case tcell.KeyDown:
		v.scrollVertical(1)
		return nil
	case tcell.KeyLeft:
		v.scrollHorizontal(-1)
		return nil
	case tcell.KeyRight:
		v.scrollHorizontal(1)
		return nil
	case tcell.KeyEsc:
		if v.searchTerm != "" {
			v.searchTerm = ""
			v.render()
			return nil
		}
	case tcell.KeyRune:
		if event.Rune() == '/' {
			v.root.ResizeItem(v.searchInput, 1, 0)
			v.searchInput.SetText("")
			v.app.SetFocus(v.searchInput)
			return nil
		}
	}

	return event
}

func (v *Viewer) rebuildLayout(width int) bool {
	if width <= 0 {
		return false
	}

	cols := maxInt(1, width/minPaneWidth)
	cols = minInt(cols, len(v.panes))
	if cols == 0 {
		cols = 1
	}

	if cols == v.layoutCols && width == v.layoutWidth {
		return false
	}

	v.layoutCols = cols
	v.layoutWidth = width
	v.layout.Clear()

	for start := 0; start < len(v.panes); start += cols {
		row := tview.NewFlex().SetDirection(tview.FlexColumn)
		end := minInt(start+cols, len(v.panes))
		for _, pane := range v.panes[start:end] {
			row.AddItem(pane, 0, 1, false)
		}
		v.layout.AddItem(row, 0, 1, false)
	}

	return true
}

func (v *Viewer) render() {
	v.rowOffset = minInt(v.rowOffset, v.maxRowOffset())
	v.colOffset = maxInt(0, v.colOffset)
	for index, pane := range v.panes {
		pane.SetText(v.renderDocument(index))
	}
	searchStatus := "none"
	if v.searchTerm != "" {
		searchStatus = tview.Escape(v.searchTerm)
	}
	v.status.SetText(fmt.Sprintf("[yellow]Ctrl+Q[white] quit  [yellow]/[white] search  [yellow]Esc[white] reset search  [yellow]Arrows[white] sync scroll  [yellow]search:[white] %s", searchStatus))
}

func (v *Viewer) renderDocument(docIndex int) string {
	var builder strings.Builder
	height := v.visibleHeight()
	start := v.rowOffset
	end := len(v.comparison.Rows)
	if height > 0 {
		end = minInt(len(v.comparison.Rows), start+height)
	}

	for _, row := range v.comparison.Rows[start:end] {
		cell := row.Cells[docIndex]
		text := cropLine(cell.Text, v.colOffset)
		reference := cropLine(referenceText(row, docIndex), v.colOffset)
		prefix := rowPrefix(row, docIndex)

		lineNo := "    "
		if cell.Present {
			lineNo = fmt.Sprintf("%4d", cell.LineNo)
		}

		builder.WriteString(prefix)
		builder.WriteString("[teal]")
		builder.WriteString(lineNo)
		builder.WriteString("[-] ")
		builder.WriteString(renderStyledLine(v.comparison.Documents[docIndex].Target.RemotePath, text, reference, v.searchTerm, row.Changed))
		builder.WriteByte('\n')
	}
	return builder.String()
}

func (v *Viewer) jumpToFirstMatch() {
	if v.searchTerm == "" {
		return
	}

	target := strings.ToLower(v.searchTerm)
	for rowIndex, row := range v.comparison.Rows {
		for _, cell := range row.Cells {
			if cell.Present && strings.Contains(strings.ToLower(cell.Text), target) {
				v.rowOffset = rowIndex
				if v.rowOffset > v.maxRowOffset() {
					v.rowOffset = v.maxRowOffset()
				}
				return
			}
		}
	}
}

func (v *Viewer) scrollVertical(delta int) {
	v.rowOffset += delta
	if v.rowOffset < 0 {
		v.rowOffset = 0
	}
	if v.rowOffset > v.maxRowOffset() {
		v.rowOffset = v.maxRowOffset()
	}
}

func (v *Viewer) scrollHorizontal(delta int) {
	v.colOffset += delta
	if v.colOffset < 0 {
		v.colOffset = 0
	}
}

func (v *Viewer) maxRowOffset() int {
	maxLines := 0
	for _, document := range v.comparison.Documents {
		if len(document.Lines) > maxLines {
			maxLines = len(document.Lines)
		}
	}
	if maxLines == 0 {
		return 0
	}

	maxVisibleHeight := v.visibleHeight()
	if maxVisibleHeight <= 0 {
		return maxLines - 1
	}

	if maxLines <= maxVisibleHeight {
		return 0
	}

	return maxLines - maxVisibleHeight
}

func (v *Viewer) visibleHeight() int {
	minVisibleHeight := 0
	for _, pane := range v.panes {
		_, _, _, height := pane.GetInnerRect()
		if height <= 0 {
			continue
		}
		if minVisibleHeight == 0 || height < minVisibleHeight {
			minVisibleHeight = height
		}
	}
	return minVisibleHeight
}

func cropLine(value string, offset int) string {
	if offset <= 0 {
		return value
	}
	runes := []rune(value)
	if offset >= len(runes) {
		return ""
	}
	return string(runes[offset:])
}

func rowPrefix(row AlignedRow, docIndex int) string {
	cell := row.Cells[docIndex]
	if row.Changed && !cell.Present {
		return "[black:gold] - [-:-:-]"
	}
	if row.Changed {
		return "[black:green] + [-:-:-]"
	}
	return " "
}

func referenceText(row AlignedRow, current int) string {
	for index, cell := range row.Cells {
		if index == current {
			continue
		}
		if cell.Present {
			return cell.Text
		}
	}
	return ""
}
