// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mux

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type transferMode int

const (
	transferModeGet transferMode = iota
	transferModePut
	transferModeParallelPut
	transferModeJobs
)

var transferModeLabels = []string{"get", "put", "copy", "status"}

type transferWizard struct {
	manager *Manager
	pane    *pane

	pages   *tview.Pages
	root    tview.Primitive
	dialog  *tview.Flex
	tabs    *tview.Flex
	help    *tview.TextView
	content *tview.Flex
	side    *tview.Flex
	focus   tview.Primitive

	activeMode transferMode

	getBrowser       *transferFileBrowser
	getTargetBrowser *transferFileBrowser
	putBrowser       *transferFileBrowser
	putTargetBrowser *transferFileBrowser
	copyBrowser      *transferFileBrowser
	copyPicker       *transferTargetPicker
	transfersView    *tview.TextView
	closeCh          chan struct{}

	getTargetServer string
	getTargetPath   string
	getTargetIndex  int

	putTargetPath string

	copyTargetPath string
	copyTargets    []string
}

func newTransferWizard(m *Manager, p *pane) *transferWizard {
	w := &transferWizard{
		manager:       m,
		pane:          p,
		pages:         tview.NewPages(),
		dialog:        tview.NewFlex().SetDirection(tview.FlexRow),
		tabs:          tview.NewFlex(),
		help:          tview.NewTextView().SetDynamicColors(true).SetWrap(true),
		content:       tview.NewFlex(),
		side:          tview.NewFlex().SetDirection(tview.FlexRow),
		transfersView: tview.NewTextView().SetDynamicColors(true).SetWrap(true),
		closeCh:       make(chan struct{}),
	}

	w.dialog.SetBorder(true)
	w.dialog.SetTitle("file transfer")
	w.dialog.SetBorderColor(tcell.ColorGreen)
	w.dialog.SetTitleColor(tcell.ColorGreen)

	w.help.SetText("[yellow]Ctrl+N/Ctrl+P[-]: switch tab  [yellow]Ctrl+S[-]: run  [yellow]Ctrl+D[-]: change get target  [yellow]Space[-]: select source  [yellow]Enter[-]: select/open  [yellow]Esc[-]: close")

	w.initTabs()
	w.initBrowsers()

	base := tview.NewFlex().SetDirection(tview.FlexRow)
	base.AddItem(w.tabs, 1, 0, false)
	base.AddItem(w.content, 0, 1, true)
	base.AddItem(w.help, 2, 0, false)
	w.dialog.AddItem(base, 0, 1, true)

	w.pages.AddPage("main", w.dialog, true, true)
	w.root = centered(w.pages, 78, 28)

	go w.runTransfersTicker()
	w.setMode(transferModeGet)
	return w
}

func (w *transferWizard) initTabs() {
	for _, label := range transferModeLabels {
		tabLabel := label
		button := tview.NewButton(tabLabel)
		button.SetSelectedFunc(func() { w.setMode(modeByLabel(tabLabel)) })
		w.tabs.AddItem(button, 0, 1, false)
	}
}

func (w *transferWizard) initBrowsers() {
	w.getBrowser = newTransferFileBrowser(
		func(current string) (string, []transferFileEntry, error) {
			client, err := w.pane.session.OpenSFTP()
			if err != nil {
				return "", nil, err
			}
			defer client.Close()
			return loadRemoteEntries(client, current)
		},
		w.restore,
		func([]string) {},
		func(delta int) { w.shiftTab(delta) },
	)
	w.getBrowser.table.SetInputCapture(w.wrapTransferInput(w.getBrowser.handleInput))
	w.getBrowser.onFocusNav = w.moveFocus

	w.getTargetBrowser = newTransferFileBrowser(
		loadLocalEntries,
		nil,
		func([]string) {},
		func(delta int) { w.shiftTab(delta) },
	)
	w.getTargetBrowser.table.SetInputCapture(w.wrapTransferInput(w.getTargetBrowser.handleInput))
	w.getTargetBrowser.onFocusNav = w.moveFocus
	w.getTargetBrowser.onSelectPath = func(selected string) {
		w.getTargetPath = selected
		w.refreshTargetBrowsers()
	}

	w.putBrowser = newTransferFileBrowser(
		loadLocalEntries,
		w.restore,
		func([]string) {},
		func(delta int) { w.shiftTab(delta) },
	)
	w.putBrowser.table.SetInputCapture(w.wrapTransferInput(w.putBrowser.handleInput))
	w.putBrowser.onFocusNav = w.moveFocus

	w.putTargetBrowser = newTransferFileBrowser(
		func(current string) (string, []transferFileEntry, error) {
			client, err := w.pane.session.OpenSFTP()
			if err != nil {
				return "", nil, err
			}
			defer client.Close()
			return loadRemoteEntries(client, current)
		},
		nil,
		func([]string) {},
		func(delta int) { w.shiftTab(delta) },
	)
	w.putTargetBrowser.table.SetInputCapture(w.wrapTransferInput(w.putTargetBrowser.handleInput))
	w.putTargetBrowser.onFocusNav = w.moveFocus
	w.putTargetBrowser.onSelectPath = func(selected string) {
		w.putTargetPath = selected
	}

	w.copyBrowser = newTransferFileBrowser(
		func(current string) (string, []transferFileEntry, error) {
			client, err := w.pane.session.OpenSFTP()
			if err != nil {
				return "", nil, err
			}
			defer client.Close()
			return loadRemoteEntries(client, current)
		},
		w.restore,
		func([]string) {},
		func(delta int) { w.shiftTab(delta) },
	)
	w.copyBrowser.table.SetInputCapture(w.wrapTransferInput(w.copyBrowser.handleInput))
	w.copyBrowser.onFocusNav = w.moveFocus
	w.copyBrowser.setTitle("source")
	w.copyBrowser.setHostLabel(w.pane.server)
	w.copyBrowser.setPathLabel("source path")

	w.copyPicker = newTransferTargetPicker(w.connectedTargetServers(true), w.copyTargets, func(targets []string) {
		w.copyTargets = append([]string(nil), targets...)
	})
	w.copyPicker.onFocusNav = w.moveFocus
	w.copyPicker.SetInputCapture(w.wrapTransferInput(w.copyPicker.handleInput))

	w.transfersView.SetInputCapture(w.wrapTransferInput(nil))
}

func modeByLabel(label string) transferMode {
	for i, value := range transferModeLabels {
		if value == label {
			return transferMode(i)
		}
	}
	return transferModeGet
}

func (w *transferWizard) primitive() tview.Primitive {
	return w.root
}

func (w *transferWizard) focusTarget() tview.Primitive {
	return w.focus
}

func (w *transferWizard) setFocus(target tview.Primitive) {
	if target == nil {
		return
	}
	w.focus = target
	w.manager.app.SetFocus(target)
}

func (w *transferWizard) focusables() []tview.Primitive {
	switch w.activeMode {
	case transferModeGet:
		return []tview.Primitive{w.getBrowser.table, w.getTargetBrowser.table}
	case transferModePut:
		return []tview.Primitive{w.putBrowser.table, w.putTargetBrowser.table}
	case transferModeParallelPut:
		items := []tview.Primitive{w.copyBrowser.table, w.copyPicker}
		if w.side.GetItemCount() > 1 {
			if form, ok := w.side.GetItem(1).(*tview.Form); ok {
				items = append(items, form)
			}
		}
		return items
	case transferModeJobs:
		return []tview.Primitive{w.transfersView}
	default:
		return nil
	}
}

func (w *transferWizard) moveFocus(delta int) {
	items := w.focusables()
	if len(items) == 0 {
		return
	}
	current := 0
	for i, item := range items {
		if item == w.focus {
			current = i
			break
		}
	}
	next := current + delta
	if next < 0 {
		next = len(items) - 1
	}
	if next >= len(items) {
		next = 0
	}
	w.setFocus(items[next])
}

func (w *transferWizard) setMode(mode transferMode) {
	w.activeMode = mode
	w.renderTabs()
	w.rebuildContent()
	w.setFocus(w.focus)
}

func (w *transferWizard) shiftTab(delta int) {
	next := int(w.activeMode) + delta
	if next < 0 {
		next = len(transferModeLabels) - 1
	}
	if next >= len(transferModeLabels) {
		next = 0
	}
	w.setMode(transferMode(next))
}

func (w *transferWizard) wrapTransferInput(next func(*tcell.EventKey) *tcell.EventKey) func(*tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		event = w.captureBaseInput(event)
		if event == nil {
			return nil
		}
		if next != nil {
			return next(event)
		}
		return event
	}
}

func (w *transferWizard) cycleGetTarget() {
	options := append([]string{"local"}, w.connectedTargetServers(false)...)
	if len(options) == 0 {
		return
	}
	w.getTargetIndex++
	if w.getTargetIndex >= len(options) {
		w.getTargetIndex = 0
	}
	w.setMode(transferModeGet)
}

func (w *transferWizard) renderTabs() {
	for i := 0; i < w.tabs.GetItemCount(); i++ {
		item := w.tabs.GetItem(i)
		button, ok := item.(*tview.Button)
		if !ok {
			continue
		}
		active := transferMode(i) == w.activeMode
		button.SetLabel(" " + transferModeLabels[i] + " ")
		if active {
			button.SetStyle(tcell.StyleDefault.Background(tcell.ColorGreen).Foreground(tcell.ColorBlack))
			button.SetLabelColor(tcell.ColorBlack)
			button.SetBackgroundColorActivated(tcell.ColorGreen)
			button.SetLabelColorActivated(tcell.ColorBlack)
			continue
		}
		button.SetStyle(tcell.StyleDefault.Background(tcell.ColorTeal).Foreground(tcell.ColorBlack))
		button.SetLabelColor(tcell.ColorBlack)
		button.SetBackgroundColorActivated(tcell.ColorYellow)
		button.SetLabelColorActivated(tcell.ColorBlack)
	}
}

func (w *transferWizard) rebuildContent() {
	switch w.activeMode {
	case transferModeGet:
		w.buildGetContent()
		w.focus = w.getBrowser.table
	case transferModePut:
		w.buildPutContent()
		w.focus = w.putBrowser.table
	case transferModeParallelPut:
		w.buildCopyContent()
		w.focus = w.copyBrowser.table
	case transferModeJobs:
		w.buildStatusContent()
		w.focus = w.transfersView
	}
}

func (w *transferWizard) setContent(left, right tview.Primitive) {
	w.setContentWithMarker(left, right, "")
}

func (w *transferWizard) setContentWithMarker(left, right tview.Primitive, marker string) {
	w.content.Clear()
	if left != nil {
		w.content.AddItem(left, 0, 1, true)
	}
	if left != nil && right != nil && marker != "" {
		middle := tview.NewFlex().SetDirection(tview.FlexRow)
		markerView := tview.NewTextView().SetDynamicColors(true).SetWrap(false).SetTextAlign(tview.AlignCenter)
		markerView.SetText(marker)
		middle.AddItem(nil, 2, 0, false)
		middle.AddItem(markerView, 1, 0, false)
		middle.AddItem(nil, 0, 1, false)
		w.content.AddItem(middle, 3, 0, false)
	}
	if right != nil {
		w.content.AddItem(right, 0, 1, false)
	}
}

func (w *transferWizard) buildGetContent() {
	options := append([]string{"local"}, w.connectedTargetServers(false)...)
	if w.getTargetIndex >= len(options) {
		w.getTargetIndex = 0
	}
	if len(options) > 0 {
		selected := options[w.getTargetIndex]
		if selected == "local" {
			w.getTargetServer = ""
		} else {
			w.getTargetServer = selected
		}
	}

	w.getBrowser.setTitle("source")
	w.getBrowser.setHostLabel(w.pane.server)
	w.getBrowser.setPathLabel("source path")
	w.getTargetBrowser.setTitle("target")
	w.getTargetBrowser.setHostLabel(w.currentTargetLabel())
	w.getTargetBrowser.setPathLabel("target path")
	w.refreshTargetBrowsers()
	w.setContentWithMarker(w.getBrowser.root, w.getTargetBrowser.root, "[yellow] > [-]")
}

func (w *transferWizard) buildPutContent() {
	w.putBrowser.setTitle("source")
	w.putBrowser.setHostLabel("local")
	w.putBrowser.setPathLabel("source path")
	w.putTargetBrowser.setTitle("target")
	w.putTargetBrowser.setHostLabel(w.pane.server)
	w.putTargetBrowser.setPathLabel("target path")
	w.refreshTargetBrowsers()
	w.setContentWithMarker(w.putBrowser.root, w.putTargetBrowser.root, "[yellow] > [-]")
}

func (w *transferWizard) buildCopyContent() {
	w.side.Clear()

	controls := tview.NewForm()
	styleTransferForm(controls)
	controls.SetBorder(true)
	controls.SetTitle("copy settings")
	controls.SetBorderColor(tcell.ColorGreen)
	controls.SetTitleColor(tcell.ColorYellow)
	controls.SetBackgroundColor(tcell.ColorBlack)
	controls.SetButtonsAlign(tview.AlignLeft)
	controls.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		event = w.captureBaseInput(event)
		if event == nil {
			return nil
		}
		switch event.Key() {
		case tcell.KeyTab:
			w.moveFocus(1)
			return nil
		case tcell.KeyBacktab:
			w.moveFocus(-1)
			return nil
		default:
			return event
		}
	})

	if w.copyTargetPath == "" {
		w.copyTargetPath = "."
	}
	controls.AddInputField("Target Path", w.copyTargetPath, 0, nil, func(text string) {
		w.copyTargetPath = text
	})
	if item := controls.GetFormItem(0); item != nil {
		if input, ok := item.(*tview.InputField); ok {
			input.SetFieldBackgroundColor(tcell.ColorAqua)
			input.SetFieldTextColor(tcell.ColorBlack)
			input.SetLabelColor(tcell.ColorYellow)
			input.SetPlaceholder(".")
			input.SetPlaceholderTextColor(tcell.ColorWhite)
		}
	}

	w.copyPicker.SetBorder(true)
	w.copyPicker.SetTitle("target panes")
	w.copyPicker.SetBorderColor(tcell.ColorGreen)
	w.copyPicker.SetTitleColor(tcell.ColorYellow)
	w.copyPicker.SetSelected(w.copyTargets)
	w.side.AddItem(w.copyPicker, 0, 1, false)
	w.side.AddItem(controls, 5, 0, false)
	w.setContent(w.copyBrowser.root, w.side)
}

func (w *transferWizard) buildStatusContent() {
	w.renderTransfers()
	w.transfersView.SetBorder(true)
	w.transfersView.SetTitle("status")
	w.transfersView.SetBorderColor(tcell.ColorGreen)
	w.transfersView.SetTitleColor(tcell.ColorYellow)
	w.setContent(w.transfersView, nil)
}

func (w *transferWizard) captureBaseInput(event *tcell.EventKey) *tcell.EventKey {
	if event == nil {
		return nil
	}
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyCtrlC:
		w.restore()
		return nil
	case tcell.KeyCtrlS:
		w.startTransfer()
		return nil
	case tcell.KeyCtrlD:
		if w.activeMode == transferModeGet {
			w.cycleGetTarget()
			return nil
		}
		return event
	case tcell.KeyCtrlN:
		w.shiftTab(1)
		return nil
	case tcell.KeyCtrlP:
		w.shiftTab(-1)
		return nil
	case tcell.KeyTab:
		return event
	case tcell.KeyBacktab:
		return event
	case tcell.KeyLeft:
		if event.Modifiers()&tcell.ModShift != 0 {
			w.shiftTab(-1)
			return nil
		}
		return event
	case tcell.KeyRight:
		if event.Modifiers()&tcell.ModShift != 0 {
			w.shiftTab(1)
			return nil
		}
		return event
	default:
		return event
	}
}

func (w *transferWizard) restore() {
	select {
	case <-w.closeCh:
	default:
		close(w.closeCh)
	}
	w.pane.primitive = nil
	w.pane.focusTarget = nil
	w.manager.refreshMainPage()
}

func (w *transferWizard) runTransfersTicker() {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.manager.app.QueueUpdateDraw(func() {
				w.renderTransfers()
			})
		case <-w.closeCh:
			return
		}
	}
}

func (w *transferWizard) renderTransfers() {
	if w.transfersView == nil {
		return
	}
	jobs := w.manager.transferJobs()
	if len(jobs) == 0 {
		w.transfersView.SetText("[yellow]No transfers yet[-]")
		return
	}

	lines := make([]string, 0, len(jobs))
	for _, job := range jobs {
		lines = append(lines, fmt.Sprintf(
			"[yellow]#%d[-] %s %s -> %s %s %s",
			job.ID,
			job.Mode,
			tview.Escape(job.Source),
			tview.Escape(job.Target),
			renderTransferProgress(job.DoneItems, job.TotalItems),
			tview.Escape(job.statusText()),
		))
	}
	w.transfersView.SetText(strings.Join(lines, "\n"))
}

func (w *transferWizard) refreshTargetBrowsers() {
	if w.getTargetServer == "" {
		w.getTargetBrowser.load = loadLocalEntries
	} else {
		targetPane := w.paneByServer(w.getTargetServer)
		if targetPane != nil {
			w.getTargetBrowser.load = func(current string) (string, []transferFileEntry, error) {
				client, err := targetPane.session.OpenSFTP()
				if err != nil {
					return "", nil, err
				}
				defer client.Close()
				return loadRemoteEntries(client, current)
			}
		}
	}

	w.getTargetBrowser.onSelectPath = func(selected string) {
		w.getTargetPath = selected
	}
	w.getTargetBrowser.reload(w.getTargetPath)

	w.putTargetBrowser.onSelectPath = func(selected string) {
		w.putTargetPath = selected
	}
	w.putTargetBrowser.reload(w.putTargetPath)
}

func (w *transferWizard) currentSources() []string {
	switch w.activeMode {
	case transferModeGet:
		return w.getBrowser.selectedPaths()
	case transferModePut:
		return w.putBrowser.selectedPaths()
	case transferModeParallelPut:
		return w.copyBrowser.selectedPaths()
	default:
		return nil
	}
}

func (w *transferWizard) currentTargetLabel() string {
	switch w.activeMode {
	case transferModeGet:
		if w.getTargetServer == "" {
			return "local"
		}
		return w.getTargetServer
	case transferModePut:
		return w.pane.server
	case transferModeParallelPut:
		if len(w.copyTargets) == 0 {
			return "(select panes)"
		}
		return strings.Join(w.copyTargets, ", ")
	default:
		return ""
	}
}

func (w *transferWizard) currentTargetPath() string {
	switch w.activeMode {
	case transferModeGet:
		if w.getTargetPath == "" {
			return "."
		}
		return w.getTargetPath
	case transferModePut:
		if w.putTargetPath == "" {
			return "."
		}
		return w.putTargetPath
	case transferModeParallelPut:
		if w.copyTargetPath == "" {
			return "."
		}
		return w.copyTargetPath
	default:
		return ""
	}
}

func (w *transferWizard) openOverlay(name, title string, body tview.Primitive, focus tview.Primitive, width, height int) {
	frame := tview.NewFlex().SetDirection(tview.FlexRow)
	frame.SetBorder(true)
	frame.SetTitle(title)
	frame.AddItem(body, 0, 1, true)
	w.pages.RemovePage(name)
	w.pages.AddPage(name, centered(frame, width, height), true, true)
	w.manager.app.SetFocus(focus)
	w.focus = focus
}

func (w *transferWizard) closeOverlay(name string) {
	w.pages.RemovePage(name)
	w.manager.app.SetFocus(w.focus)
}

func (w *transferWizard) showResult(err error) {
	view := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	if err != nil {
		view.SetText(fmt.Sprintf("[red]transfer failed[-]\n\n%s\n\n[gray]Enter/Esc: close[-]", tview.Escape(err.Error())))
		w.manager.updateStatus(fmt.Sprintf("[red]transfer failed[-]: %v", err))
	} else {
		view.SetText("[green]transfer completed[-]\n\n[gray]Enter/Esc: close[-]")
		w.manager.updateStatus("[green]transfer completed[-]")
	}

	view.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event == nil {
			return nil
		}
		switch event.Key() {
		case tcell.KeyEnter, tcell.KeyEsc, tcell.KeyCtrlC:
			w.closeOverlay("result")
			return nil
		default:
			return event
		}
	})
	w.openOverlay("result", "transfer result", view, view, 46, 9)
}
