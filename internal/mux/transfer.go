// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mux

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sshcmd "github.com/blacknon/lssh/internal/ssh"
	"github.com/gdamore/tcell/v2"
	"github.com/pkg/sftp"
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

type transferJob struct {
	ID         int
	Mode       string
	Source     string
	Target     string
	Status     string
	DoneItems  int
	TotalItems int
	Err        string
}

type transferWizard struct {
	manager *Manager
	pane    *pane

	pages   *tview.Pages
	root    tview.Primitive
	dialog  *tview.Flex
	tabs    *tview.Flex
	summary *tview.TextView
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
	prefixWait       bool

	getTargetServer string
	getTargetPath   string
	getTargetIndex  int

	putTargetPath string

	copyTargetPath string
	copyTargets    []string
}

type transferFileEntry struct {
	Name  string
	Path  string
	IsDir bool
}

type transferFileBrowser struct {
	root       *tview.Flex
	headerView *tview.TextView
	pathView   *tview.TextView
	helpView   *tview.TextView
	table      *tview.Table

	load         func(string) (string, []transferFileEntry, error)
	onCancel     func()
	onChange     func([]string)
	onTabNav     func(int)
	onSelectPath func(string)

	current   string
	entries   []transferFileEntry
	selected  map[string]transferFileEntry
	selecting bool
	title     string
	pathLabel string
	hostLabel string
}

type transferTargetPicker struct {
	*tview.List
	all      []string
	selected map[string]struct{}
	onChange func([]string)
}

type transferSFTPConn struct {
	client  *sftp.Client
	closeFn func() error
}

func (c *transferSFTPConn) Close() error {
	if c == nil {
		return nil
	}

	var firstErr error
	if c.client != nil {
		if err := c.client.Close(); firstErr == nil && err != nil {
			firstErr = err
		}
	}
	if c.closeFn != nil {
		if err := c.closeFn(); firstErr == nil && err != nil {
			firstErr = err
		}
	}

	return firstErr
}

func (w *transferWizard) openTransferSFTPForPane(p *pane) (*transferSFTPConn, error) {
	if w == nil || w.manager == nil || p == nil {
		return nil, fmt.Errorf("sftp unavailable")
	}

	run := &sshcmd.Run{
		ServerList: []string{p.server},
		Conf:       w.manager.conf,
	}
	run.CreateAuthMethodMap()

	connect, err := run.CreateSshConnectDirect(p.server)
	if err != nil {
		return nil, err
	}

	client, err := connect.OpenSFTP()
	if err != nil {
		_ = connect.Close()
		return nil, err
	}

	return &transferSFTPConn{
		client:  client,
		closeFn: connect.Close,
	}, nil
}

func newTransferWizard(m *Manager, p *pane) *transferWizard {
	w := &transferWizard{
		manager:       m,
		pane:          p,
		pages:         tview.NewPages(),
		dialog:        tview.NewFlex().SetDirection(tview.FlexRow),
		tabs:          tview.NewFlex(),
		summary:       tview.NewTextView().SetDynamicColors(true).SetWrap(true),
		help:          tview.NewTextView().SetDynamicColors(true).SetWrap(true),
		content:       tview.NewFlex(),
		side:          tview.NewFlex().SetDirection(tview.FlexRow),
		transfersView: tview.NewTextView().SetDynamicColors(true).SetWrap(true),
		closeCh:       make(chan struct{}),
	}

	w.dialog.SetBorder(true)
	w.dialog.SetTitle("file transfer")
	w.help.SetText("[yellow]Ctrl+N/Ctrl+P[-]: switch tab  [yellow]Ctrl+S[-]: run  [yellow]Ctrl+D[-]: change get target  [yellow]Space[-]: select source  [yellow]Enter[-]: select/open  [yellow]Esc[-]: close")

	for _, label := range transferModeLabels {
		tabLabel := label
		button := tview.NewButton(tabLabel)
		button.SetSelectedFunc(func() { w.setMode(modeByLabel(tabLabel)) })
		w.tabs.AddItem(button, 0, 1, false)
	}

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

	w.getTargetBrowser = newTransferFileBrowser(
		loadLocalEntries,
		nil,
		func([]string) {},
		func(delta int) { w.shiftTab(delta) },
	)
	w.getTargetBrowser.table.SetInputCapture(w.wrapTransferInput(w.getTargetBrowser.handleInput))
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
	w.copyBrowser.SetTitle("source")
	w.copyBrowser.SetHostLabel(w.pane.server)
	w.copyBrowser.SetPathLabel("source path")

	w.copyPicker = newTransferTargetPicker(w.connectedTargetServers(true), w.copyTargets, func(targets []string) {
		w.copyTargets = append([]string(nil), targets...)
	})
	w.copyPicker.SetInputCapture(w.wrapTransferInput(w.copyPicker.handleInput))

	w.transfersView.SetInputCapture(w.wrapTransferInput(nil))

	w.content.AddItem(w.getBrowser.root, 0, 2, true)
	w.content.AddItem(w.side, 0, 1, false)

	base := tview.NewFlex().SetDirection(tview.FlexRow)
	base.AddItem(w.tabs, 1, 0, false)
	base.AddItem(w.summary, 1, 0, false)
	base.AddItem(w.content, 0, 1, true)
	base.AddItem(w.help, 1, 0, false)

	w.dialog.SetBorder(true)
	w.dialog.SetBorderColor(tcell.ColorGreen)
	w.dialog.SetTitleColor(tcell.ColorGreen)
	w.dialog.AddItem(base, 0, 1, true)

	w.pages.AddPage("main", w.dialog, true, true)
	w.root = centered(w.pages, 78, 28)

	go w.runTransfersTicker()
	w.setMode(transferModeGet)
	return w
}

func modeByLabel(label string) transferMode {
	for i, value := range transferModeLabels {
		if value == label {
			return transferMode(i)
		}
	}
	return transferModeGet
}

func (w *transferWizard) Primitive() tview.Primitive {
	return w.root
}

func (w *transferWizard) FocusTarget() tview.Primitive {
	return w.focus
}

func (w *transferWizard) setMode(mode transferMode) {
	w.activeMode = mode
	w.renderTabs()
	w.rebuildSide()
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
		} else {
			button.SetStyle(tcell.StyleDefault.Background(tcell.ColorTeal).Foreground(tcell.ColorBlack))
			button.SetLabelColor(tcell.ColorBlack)
			button.SetBackgroundColorActivated(tcell.ColorYellow)
			button.SetLabelColorActivated(tcell.ColorBlack)
		}
	}
}

func (w *transferWizard) rebuildSide() {
	switch w.activeMode {
	case transferModeGet:
		w.buildGetSide()
		w.focus = w.getBrowser.table
	case transferModePut:
		w.buildPutSide()
		w.focus = w.putBrowser.table
	case transferModeParallelPut:
		w.buildCopySide()
		w.focus = w.copyBrowser.table
	case transferModeJobs:
		w.buildTransfersSide()
		w.focus = w.transfersView
	}
}

func (w *transferWizard) setContent(left, right tview.Primitive) {
	w.setDirectionalContent(left, right, "")
}

func (w *transferWizard) setDirectionalContent(left, right tview.Primitive, marker string) {
	w.content.Clear()
	if left != nil {
		w.content.AddItem(left, 0, 1, true)
	}
	if left != nil && right != nil && marker != "" {
		middle := tview.NewTextView().SetDynamicColors(true).SetWrap(false).SetTextAlign(tview.AlignCenter)
		middle.SetText(marker)
		w.content.AddItem(middle, 3, 0, false)
	}
	if right != nil {
		w.content.AddItem(right, 0, 1, false)
	}
}

func (w *transferWizard) buildGetSide() {
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
	w.getBrowser.SetTitle("source")
	w.getBrowser.SetHostLabel(w.pane.server)
	w.getBrowser.SetPathLabel("source path")
	w.getTargetBrowser.SetTitle("target")
	w.getTargetBrowser.SetHostLabel(w.currentTargetLabel())
	w.getTargetBrowser.SetPathLabel("target path")
	w.refreshTargetBrowsers()
	w.setDirectionalContent(w.getBrowser.root, w.getTargetBrowser.root, "[yellow] > [-]")
}

func (w *transferWizard) buildPutSide() {
	w.putBrowser.SetTitle("source")
	w.putBrowser.SetHostLabel("local")
	w.putBrowser.SetPathLabel("source path")
	w.putTargetBrowser.SetTitle("target")
	w.putTargetBrowser.SetHostLabel(w.pane.server)
	w.putTargetBrowser.SetPathLabel("target path")
	w.refreshTargetBrowsers()
	w.setDirectionalContent(w.putBrowser.root, w.putTargetBrowser.root, "[yellow] > [-]")
}

func (w *transferWizard) buildCopySide() {
	w.side.Clear()

	controls := tview.NewForm()
	styleTransferForm(controls)
	controls.SetBorder(true)
	controls.SetTitle("copy settings")
	controls.SetButtonsAlign(tview.AlignLeft)
	controls.SetInputCapture(w.captureBaseInput)

	if w.copyTargetPath == "" {
		w.copyTargetPath = "."
	}
	controls.AddInputField("Target Path", w.copyTargetPath, 0, nil, func(text string) {
		w.copyTargetPath = text
	})
	if item := controls.GetFormItem(0); item != nil {
		if input, ok := item.(*tview.InputField); ok {
			input.SetFieldBackgroundColor(tcell.ColorTeal)
			input.SetFieldTextColor(tcell.ColorBlack)
			input.SetLabelColor(tcell.ColorYellow)
		}
	}

	pickerFrame := tview.NewFlex().SetDirection(tview.FlexRow)
	pickerTitle := tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	pickerTitle.SetText("[yellow]target panes[-]")
	w.copyPicker.SetSelected(w.copyTargets)
	pickerFrame.AddItem(pickerTitle, 1, 0, false)
	pickerFrame.AddItem(w.copyPicker, 0, 1, false)

	w.side.AddItem(pickerFrame, 8, 0, false)
	w.side.AddItem(controls, 0, 1, false)
	w.setContent(w.copyBrowser.root, w.side)
}

func (w *transferWizard) buildTransfersSide() {
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
		w.shiftTab(1)
		return nil
	case tcell.KeyBacktab:
		w.shiftTab(-1)
		return nil
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
			tview.Escape(job.StatusText()),
		))
	}
	w.transfersView.SetText(strings.Join(lines, "\n"))
}

func (j *transferJob) StatusText() string {
	switch j.Status {
	case "done":
		return "done"
	case "error":
		if j.Err != "" {
			return "error: " + j.Err
		}
		return "error"
	default:
		return "running"
	}
}

func renderTransferProgress(done, total int) string {
	if total <= 0 {
		total = 1
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}
	width := 12
	filled := done * width / total
	return "[" + strings.Repeat("=", filled) + strings.Repeat(" ", width-filled) + fmt.Sprintf("] %d/%d", done, total)
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
		return w.getBrowser.SelectedPaths()
	case transferModePut:
		return w.putBrowser.SelectedPaths()
	case transferModeParallelPut:
		return w.copyBrowser.SelectedPaths()
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

func (w *transferWizard) startTransfer() {
	if err := w.launchTransferJobs(); err != nil {
		w.manager.updateStatus(fmt.Sprintf("[red]transfer start failed[-]: %v", err))
		w.showResult(err)
		return
	}
	w.setMode(transferModeJobs)
	w.manager.updateStatus("[green]transfer started[-]")
}

func (m *Manager) newTransferJob(mode, source, target string, total int) *transferJob {
	m.transferMu.Lock()
	defer m.transferMu.Unlock()

	m.nextTransferID++
	if total <= 0 {
		total = 1
	}
	job := &transferJob{
		ID:         m.nextTransferID,
		Mode:       mode,
		Source:     source,
		Target:     target,
		Status:     "running",
		TotalItems: total,
	}
	m.transfers = append([]*transferJob{job}, m.transfers...)
	return job
}

func (m *Manager) updateTransferJob(job *transferJob, update func(*transferJob)) {
	if job == nil || update == nil {
		return
	}
	m.transferMu.Lock()
	update(job)
	m.transferMu.Unlock()
}

func (m *Manager) transferJobs() []*transferJob {
	m.transferMu.Lock()
	defer m.transferMu.Unlock()

	out := make([]*transferJob, 0, len(m.transfers))
	for _, job := range m.transfers {
		if job == nil {
			continue
		}
		copyJob := *job
		out = append(out, &copyJob)
	}
	return out
}

func (w *transferWizard) connectedTargetServers(includeCurrent bool) []string {
	seen := map[string]struct{}{}
	servers := []string{}
	for _, pg := range w.manager.sessionPages {
		for _, p := range pg.panes {
			if p == nil || p.session == nil || p.term == nil || p.failed || p.exited || p.transient {
				continue
			}
			if !includeCurrent && p == w.pane {
				continue
			}
			if _, ok := seen[p.server]; ok {
				continue
			}
			seen[p.server] = struct{}{}
			servers = append(servers, p.server)
		}
	}
	sort.Strings(servers)
	return servers
}

func (w *transferWizard) paneByServer(server string) *pane {
	for _, pg := range w.manager.sessionPages {
		for _, p := range pg.panes {
			if p == nil || p.server != server || p.session == nil || p.term == nil || p.failed || p.exited || p.transient {
				continue
			}
			return p
		}
	}
	return nil
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

func (w *transferWizard) launchTransferJobs() error {
	sources := w.currentSources()
	if len(sources) == 0 {
		return fmt.Errorf("source is not selected")
	}

	switch w.activeMode {
	case transferModeGet:
		for _, source := range sources {
			sourcePath := source
			targetLabel := w.currentTargetLabel() + ":" + w.getTargetPath
			job := w.manager.newTransferJob("get", sourcePath, targetLabel, 1)
			go w.runGetJob(job, sourcePath)
		}
		return nil

	case transferModePut:
		for _, source := range sources {
			sourcePath := source
			job := w.manager.newTransferJob("put", sourcePath, w.pane.server+":"+w.putTargetPath, 1)
			go w.runPutJob(job, sourcePath)
		}
		return nil

	case transferModeParallelPut:
		if len(w.copyTargets) == 0 {
			return fmt.Errorf("target panes are not selected")
		}
		for _, server := range w.copyTargets {
			for _, source := range sources {
				sourcePath := source
				targetServer := server
				job := w.manager.newTransferJob("copy", sourcePath, targetServer+":"+w.copyTargetPath, 1)
				go w.runCopyJob(job, sourcePath, targetServer)
			}
		}
		return nil

	default:
		return fmt.Errorf("unknown transfer mode")
	}
}

func (w *transferWizard) runGetJob(job *transferJob, source string) {
	srcConn, err := w.openTransferSFTPForPane(w.pane)
	if err != nil {
		w.finishTransferJob(job, err)
		return
	}
	defer srcConn.Close()

	if w.getTargetServer == "" {
		err = copyRemotePathToLocal(srcConn.client, source, w.getTargetPath)
		w.finishTransferJob(job, err)
		return
	}

	targetPane := w.paneByServer(w.getTargetServer)
	if targetPane == nil {
		w.finishTransferJob(job, fmt.Errorf("target pane %s is not connected", w.getTargetServer))
		return
	}

	dstConn, err := w.openTransferSFTPForPane(targetPane)
	if err != nil {
		w.finishTransferJob(job, err)
		return
	}
	defer dstConn.Close()

	err = copyRemotePathToRemote(srcConn.client, dstConn.client, source, w.getTargetPath)
	w.finishTransferJob(job, err)
}

func (w *transferWizard) runPutJob(job *transferJob, source string) {
	dstConn, err := w.openTransferSFTPForPane(w.pane)
	if err != nil {
		w.finishTransferJob(job, err)
		return
	}
	defer dstConn.Close()

	err = copyLocalPathToRemote(dstConn.client, source, w.putTargetPath)
	w.finishTransferJob(job, err)
}

func (w *transferWizard) runCopyJob(job *transferJob, source, server string) {
	target := w.paneByServer(server)
	if target == nil {
		w.finishTransferJob(job, fmt.Errorf("target pane %s is not connected", server))
		return
	}

	srcConn, err := w.openTransferSFTPForPane(w.pane)
	if err != nil {
		w.finishTransferJob(job, err)
		return
	}
	defer srcConn.Close()

	dstConn, err := w.openTransferSFTPForPane(target)
	if err != nil {
		w.finishTransferJob(job, err)
		return
	}
	defer dstConn.Close()

	err = copyRemotePathToRemote(srcConn.client, dstConn.client, source, w.copyTargetPath)
	w.finishTransferJob(job, err)
}

func (w *transferWizard) finishTransferJob(job *transferJob, err error) {
	w.manager.updateTransferJob(job, func(j *transferJob) {
		j.DoneItems = j.TotalItems
		if err != nil {
			j.Status = "error"
			j.Err = err.Error()
		} else {
			j.Status = "done"
		}
	})
}

func newTransferFileBrowser(
	load func(string) (string, []transferFileEntry, error),
	onCancel func(),
	onChange func([]string),
	onTabNav func(int),
) *transferFileBrowser {
	b := &transferFileBrowser{
		root:       tview.NewFlex().SetDirection(tview.FlexRow),
		headerView: tview.NewTextView().SetDynamicColors(true).SetWrap(false),
		pathView:   tview.NewTextView().SetDynamicColors(true).SetWrap(false),
		helpView:   tview.NewTextView().SetDynamicColors(true).SetWrap(true),
		table:      tview.NewTable().SetSelectable(true, false).SetBorders(false),
		load:       load,
		onCancel:   onCancel,
		onChange:   onChange,
		onTabNav:   onTabNav,
		selected:   map[string]transferFileEntry{},
		title:      "browser",
		pathLabel:  "path",
	}

	b.root.SetBorder(true)
	b.root.SetBorderColor(tcell.ColorGreen)
	b.root.SetTitleColor(tcell.ColorYellow)
	b.root.SetTitle("browser")
	b.helpView.SetText("[yellow]Space[-]: toggle  [yellow]Enter[-]: select/open  [yellow]BS/Left[-]: parent  [yellow]Esc[-]: close")
	b.table.SetInputCapture(b.handleInput)
	b.table.SetSelectedFunc(func(row, column int) {
		b.activateCurrent(false)
	})
	b.reload("")

	b.root.AddItem(b.headerView, 1, 0, false)
	b.root.AddItem(b.pathView, 1, 0, false)
	b.root.AddItem(b.table, 0, 1, true)
	b.root.AddItem(b.helpView, 1, 0, false)

	return b
}

func (b *transferFileBrowser) SetTitle(title string) {
	if title == "" {
		title = "browser"
	}
	b.title = title
	b.root.SetTitle(title)
}

func (b *transferFileBrowser) SetPathLabel(label string) {
	if label == "" {
		label = "path"
	}
	b.pathLabel = label
	b.updateHeader()
}

func (b *transferFileBrowser) SetHostLabel(host string) {
	b.hostLabel = host
	b.updateHeader()
}

func (b *transferFileBrowser) updateHeader() {
	if b.headerView != nil {
		b.headerView.SetText(fmt.Sprintf("[yellow]host[-]: %s", tview.Escape(b.hostLabel)))
	}
	if b.pathView != nil {
		b.pathView.SetText(fmt.Sprintf("[yellow]%s[-]: %s", tview.Escape(b.pathLabel), tview.Escape(b.current)))
	}
}

func newTransferTargetPicker(all []string, initial []string, onChange func([]string)) *transferTargetPicker {
	p := &transferTargetPicker{
		List:     tview.NewList().ShowSecondaryText(false),
		all:      append([]string(nil), all...),
		selected: map[string]struct{}{},
		onChange: onChange,
	}
	p.SetMainTextColor(tcell.ColorBlack)
	p.SetSelectedTextColor(tcell.ColorBlack)
	p.SetSelectedBackgroundColor(tcell.ColorGreen)
	p.SetHighlightFullLine(true)
	p.SetBackgroundColor(tcell.ColorTeal)
	p.SetInputCapture(p.handleInput)
	p.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		p.toggleCurrent()
	})
	p.SetSelected(initial)
	return p
}

func (p *transferTargetPicker) handleInput(event *tcell.EventKey) *tcell.EventKey {
	if event == nil {
		return nil
	}
	switch event.Key() {
	case tcell.KeyRune:
		if event.Rune() == ' ' {
			p.toggleCurrent()
			return nil
		}
	case tcell.KeyEnter:
		p.toggleCurrent()
		return nil
	}
	return event
}

func (p *transferTargetPicker) SetSelected(targets []string) {
	p.selected = map[string]struct{}{}
	for _, target := range targets {
		p.selected[target] = struct{}{}
	}
	p.refresh()
}

func (p *transferTargetPicker) Selected() []string {
	result := make([]string, 0, len(p.selected))
	for target := range p.selected {
		result = append(result, target)
	}
	sort.Strings(result)
	return result
}

func (p *transferTargetPicker) refresh() {
	current := p.GetCurrentItem()
	p.Clear()
	for _, name := range p.all {
		prefix := "[ ]"
		if _, ok := p.selected[name]; ok {
			prefix = "[x]"
		}
		p.AddItem(prefix+" "+name, "", 0, nil)
	}
	if len(p.all) == 0 {
		p.AddItem("(no connected panes)", "", 0, nil)
	}
	if current >= 0 && current < p.GetItemCount() {
		p.SetCurrentItem(current)
	}
	if p.onChange != nil {
		p.onChange(p.Selected())
	}
}

func (p *transferTargetPicker) toggleCurrent() {
	index := p.GetCurrentItem()
	if index < 0 || index >= len(p.all) {
		return
	}
	name := p.all[index]
	if _, ok := p.selected[name]; ok {
		delete(p.selected, name)
	} else {
		p.selected[name] = struct{}{}
	}
	p.refresh()
}

func (b *transferFileBrowser) SelectedPaths() []string {
	result := make([]string, 0, len(b.selected))
	for _, entry := range b.selected {
		result = append(result, entry.Path)
	}
	sort.Strings(result)
	return result
}

func (b *transferFileBrowser) reload(current string) {
	currentPath, entries, err := b.load(current)
	b.current = currentPath
	b.entries = entries
	b.updateHeader()
	b.table.Clear()

	if err != nil {
		b.table.SetCell(0, 0, tview.NewTableCell(fmt.Sprintf("load error: %v", err)).SetTextColor(tcell.ColorRed))
		return
	}

	for i, entry := range entries {
		b.table.SetCell(i, 0, b.entryCell(entry))
	}

	if len(entries) > 0 {
		b.selecting = true
		b.table.Select(0, 0)
		b.selecting = false
	}
}

func (b *transferFileBrowser) entryCell(entry transferFileEntry) *tview.TableCell {
	mark := "  "
	selected := false
	if _, ok := b.selected[entry.Path]; ok {
		mark = "x "
		selected = true
	}

	label := entry.Name
	if entry.IsDir && label != ".." {
		label += "/"
	}

	cell := tview.NewTableCell(mark + label).SetExpansion(1)
	if selected {
		cell.SetBackgroundColor(tcell.ColorTeal)
		cell.SetTextColor(tcell.ColorBlack)
		cell.SetSelectedStyle(tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorGreen))
	} else {
		cell.SetTextColor(tcell.ColorWhite)
		cell.SetSelectedStyle(tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorGreen))
	}
	return cell
}

func (b *transferFileBrowser) refreshEntries() {
	for i, entry := range b.entries {
		b.table.SetCell(i, 0, b.entryCell(entry))
	}
	if b.onChange != nil {
		b.onChange(b.SelectedPaths())
	}
}

func (b *transferFileBrowser) handleInput(event *tcell.EventKey) *tcell.EventKey {
	if event == nil {
		return nil
	}
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyCtrlC:
		if b.onCancel != nil {
			b.onCancel()
		}
		return nil
	case tcell.KeyLeft:
		if event.Modifiers()&tcell.ModShift != 0 && b.onTabNav != nil {
			b.onTabNav(-1)
			return nil
		}
		b.goParent()
		return nil
	case tcell.KeyRight:
		if event.Modifiers()&tcell.ModShift != 0 && b.onTabNav != nil {
			b.onTabNav(1)
			return nil
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		b.goParent()
		return nil
	case tcell.KeyEnter:
		b.activateCurrent(false)
		return nil
	case tcell.KeyRune:
		if event.Rune() == ' ' {
			b.activateCurrent(true)
			return nil
		}
	}
	return event
}

func (b *transferFileBrowser) goParent() {
	if b.current == "" {
		return
	}
	next := parentPathOf(b.current)
	if next == b.current {
		return
	}
	b.reload(next)
}

func (b *transferFileBrowser) activateCurrent(toggle bool) {
	row, _ := b.table.GetSelection()
	if row < 0 || row >= len(b.entries) {
		return
	}

	entry := b.entries[row]
	if entry.Name == ".." {
		b.reload(entry.Path)
		return
	}
	if entry.IsDir && !toggle {
		b.reload(entry.Path)
		return
	}
	if b.onSelectPath != nil {
		b.onSelectPath(entry.Path)
		return
	}

	if _, ok := b.selected[entry.Path]; ok {
		delete(b.selected, entry.Path)
	} else {
		b.selected[entry.Path] = entry
	}
	b.refreshEntries()
}

func loadLocalEntries(current string) (string, []transferFileEntry, error) {
	if current == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", nil, err
		}
		current = wd
	}
	current = filepath.Clean(current)

	entries := []transferFileEntry{}
	parent := filepath.Dir(current)
	if parent != current {
		entries = append(entries, transferFileEntry{Name: "..", Path: parent, IsDir: true})
	}

	contents, err := os.ReadDir(current)
	if err != nil {
		return current, nil, err
	}
	for _, entry := range sortDirEntries(contents) {
		entries = append(entries, transferFileEntry{
			Name:  entry.Name(),
			Path:  filepath.Join(current, entry.Name()),
			IsDir: entry.IsDir(),
		})
	}

	return current, entries, nil
}

func loadRemoteEntries(client *sftp.Client, current string) (string, []transferFileEntry, error) {
	if client == nil {
		return "", nil, fmt.Errorf("sftp unavailable")
	}
	if current == "" {
		wd, err := client.Getwd()
		if err != nil {
			return "", nil, err
		}
		current = wd
	}
	current, err := resolveRemotePath(client, current)
	if err != nil {
		return "", nil, err
	}

	entries := []transferFileEntry{}
	parent := path.Dir(current)
	if parent != current {
		entries = append(entries, transferFileEntry{Name: "..", Path: parent, IsDir: true})
	}

	contents, err := client.ReadDir(current)
	if err != nil {
		return current, nil, err
	}
	sort.Slice(contents, func(i, j int) bool {
		if contents[i].IsDir() != contents[j].IsDir() {
			return contents[i].IsDir()
		}
		return contents[i].Name() < contents[j].Name()
	})
	for _, entry := range contents {
		entries = append(entries, transferFileEntry{
			Name:  entry.Name(),
			Path:  path.Join(current, entry.Name()),
			IsDir: entry.IsDir(),
		})
	}

	return current, entries, nil
}

func sortDirEntries(entries []os.DirEntry) []os.DirEntry {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})
	return entries
}

func parentPathOf(current string) string {
	if strings.Contains(current, "/") && !strings.Contains(current, `\`) {
		return path.Dir(current)
	}
	return filepath.Dir(current)
}

func resolveRemotePath(client *sftp.Client, raw string) (string, error) {
	if raw == "" {
		return client.Getwd()
	}
	switch {
	case raw == "~":
		return client.Getwd()
	case strings.HasPrefix(raw, "~/"):
		wd, err := client.Getwd()
		if err != nil {
			return "", err
		}
		return path.Join(wd, raw[2:]), nil
	case !path.IsAbs(raw):
		wd, err := client.Getwd()
		if err != nil {
			return "", err
		}
		return path.Join(wd, raw), nil
	default:
		return path.Clean(raw), nil
	}
}

func copyRemotePathToLocal(client *sftp.Client, sourcePath, targetPath string) error {
	sourceInfo, err := client.Stat(sourcePath)
	if err != nil {
		return err
	}

	targetPath = resolveLocalTargetPath(targetPath, sourcePath, sourceInfo.IsDir())
	if sourceInfo.IsDir() {
		return copyRemoteDirToLocal(client, sourcePath, targetPath)
	}
	return copyRemoteFileToLocal(client, sourcePath, targetPath)
}

func copyLocalPathToRemote(client *sftp.Client, sourcePath, targetPath string) error {
	sourceInfo, err := os.Lstat(sourcePath)
	if err != nil {
		return err
	}

	targetPath, err = resolveRemoteTargetPath(client, targetPath, filepath.Base(sourcePath), sourceInfo.IsDir())
	if err != nil {
		return err
	}
	if sourceInfo.IsDir() {
		return copyLocalDirToRemote(client, sourcePath, targetPath)
	}
	return copyLocalFileToRemote(client, sourcePath, targetPath)
}

func copyRemotePathToRemote(srcClient, dstClient *sftp.Client, sourcePath, targetPath string) error {
	sourceInfo, err := srcClient.Stat(sourcePath)
	if err != nil {
		return err
	}

	targetPath, err = resolveRemoteTargetPath(dstClient, targetPath, path.Base(sourcePath), sourceInfo.IsDir())
	if err != nil {
		return err
	}
	if sourceInfo.IsDir() {
		return copyRemoteDirToRemote(srcClient, dstClient, sourcePath, targetPath)
	}
	return copyRemoteFileToRemote(srcClient, dstClient, sourcePath, targetPath)
}

func resolveLocalTargetPath(targetPath, sourcePath string, sourceIsDir bool) string {
	if targetPath == "" {
		targetPath = "."
	}
	targetPath = filepath.Clean(targetPath)

	if info, err := os.Stat(targetPath); err == nil && info.IsDir() {
		return filepath.Join(targetPath, filepath.Base(sourcePath))
	}
	if strings.HasSuffix(targetPath, string(os.PathSeparator)) {
		return filepath.Join(targetPath, filepath.Base(sourcePath))
	}
	if sourceIsDir && !strings.HasSuffix(targetPath, filepath.Base(sourcePath)) {
		return targetPath
	}
	return targetPath
}

func resolveRemoteTargetPath(client *sftp.Client, targetPath, baseName string, sourceIsDir bool) (string, error) {
	if targetPath == "" {
		targetPath = "."
	}
	resolved, err := resolveRemotePath(client, targetPath)
	if err != nil {
		return "", err
	}
	if info, err := client.Stat(resolved); err == nil && info.IsDir() {
		return path.Join(resolved, baseName), nil
	}
	if strings.HasSuffix(targetPath, "/") {
		return path.Join(resolved, baseName), nil
	}
	if sourceIsDir && !strings.HasSuffix(resolved, baseName) {
		return resolved, nil
	}
	return resolved, nil
}

func copyRemoteFileToLocal(client *sftp.Client, remotePath, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	src, err := client.Open(remotePath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func copyRemoteDirToLocal(client *sftp.Client, remotePath, localPath string) error {
	base := remotePath
	walker := client.Walk(remotePath)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return err
		}
		current := walker.Path()
		rel, err := remoteRel(base, current)
		if err != nil {
			return err
		}
		target := filepath.Join(localPath, filepath.FromSlash(rel))
		if walker.Stat().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := copyRemoteFileToLocal(client, current, target); err != nil {
			return err
		}
	}
	return nil
}

func copyLocalFileToRemote(client *sftp.Client, localPath, remotePath string) error {
	if err := client.MkdirAll(path.Dir(remotePath)); err != nil {
		return err
	}

	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := client.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func copyLocalDirToRemote(client *sftp.Client, localPath, remotePath string) error {
	base := localPath
	return filepath.Walk(localPath, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(base, current)
		if err != nil {
			return err
		}
		target := path.Join(remotePath, filepath.ToSlash(rel))
		if info.IsDir() {
			return client.MkdirAll(target)
		}
		return copyLocalFileToRemote(client, current, target)
	})
}

func copyRemoteFileToRemote(srcClient, dstClient *sftp.Client, sourcePath, targetPath string) error {
	if err := dstClient.MkdirAll(path.Dir(targetPath)); err != nil {
		return err
	}

	src, err := srcClient.Open(sourcePath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := dstClient.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func copyRemoteDirToRemote(srcClient, dstClient *sftp.Client, sourcePath, targetPath string) error {
	base := sourcePath
	walker := srcClient.Walk(sourcePath)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return err
		}
		current := walker.Path()
		rel, err := remoteRel(base, current)
		if err != nil {
			return err
		}
		target := path.Join(targetPath, rel)
		if walker.Stat().IsDir() {
			if err := dstClient.MkdirAll(target); err != nil {
				return err
			}
			continue
		}
		if err := copyRemoteFileToRemote(srcClient, dstClient, current, target); err != nil {
			return err
		}
	}
	return nil
}

func remoteRel(base, current string) (string, error) {
	base = path.Clean(base)
	current = path.Clean(current)
	if current == base {
		return ".", nil
	}
	prefix := base
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if !strings.HasPrefix(current, prefix) {
		return "", fmt.Errorf("path %s is outside %s", current, base)
	}
	return strings.TrimPrefix(current, prefix), nil
}

func styleTransferForm(form *tview.Form) {
	if form == nil {
		return
	}
	form.SetLabelColor(tcell.ColorYellow)
	form.SetFieldBackgroundColor(tcell.ColorDefault)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetButtonStyle(tcell.StyleDefault.Background(tcell.ColorTeal).Foreground(tcell.ColorBlack))
	form.SetButtonActivatedStyle(tcell.StyleDefault.Background(tcell.ColorGreen).Foreground(tcell.ColorBlack))
}

func styleTransferDropDown(dropdown *tview.DropDown) {
	if dropdown == nil {
		return
	}
	dropdown.SetLabelColor(tcell.ColorYellow)
	dropdown.SetFieldBackgroundColor(tcell.ColorDefault)
	dropdown.SetFieldTextColor(tcell.ColorWhite)
	dropdown.SetPrefixTextColor(tcell.ColorYellow)
	dropdown.SetListStyles(
		tcell.StyleDefault.Background(tcell.ColorDefault).Foreground(tcell.ColorWhite),
		tcell.StyleDefault.Background(tcell.ColorGreen).Foreground(tcell.ColorBlack),
	)
}
