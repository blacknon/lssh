// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mux

import (
	"fmt"
	"sort"
	"sync"

	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/list"
	"github.com/blacknon/tvxterm"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type selectorMode int

const (
	selectorInitial selectorMode = iota
	selectorNewPage
	selectorNewPane
	selectorSplitHorizontal
	selectorSplitVertical
)

// Manager runs the lsmux TUI.
type Manager struct {
	app               *tview.Application
	conf              conf.Config
	names             []string
	command           []string
	initial           []string
	hold              bool
	allowLayoutChange bool
	factory           SessionFactory
	bindings          map[string]keyBinding

	root   *tview.Flex
	pages  *tview.Pages
	status *tview.TextView

	sessionPages  []*page
	currentPage   *page
	selectorFocus tview.Primitive
	broadcastAll  bool
	prefixActive  bool

	nextPageID int
	nextPaneID int
	stopOnce   sync.Once
}

// NewManager creates an lsmux manager.
func NewManager(cfg conf.Config, names []string, command []string, initialHosts []string, hold bool, allowLayoutChange bool) (*Manager, error) {
	app := tview.NewApplication()
	sort.Strings(names)

	bindings := map[string]string{
		"prefix":           cfg.Mux.Prefix,
		"quit":             cfg.Mux.Quit,
		"new_page":         cfg.Mux.NewPage,
		"new_pane":         cfg.Mux.NewPane,
		"split_horizontal": cfg.Mux.SplitHorizontal,
		"split_vertical":   cfg.Mux.SplitVertical,
		"next_pane":        cfg.Mux.NextPane,
		"next_page":        cfg.Mux.NextPage,
		"prev_page":        cfg.Mux.PrevPage,
		"page_list":        cfg.Mux.PageList,
		"close_pane":       cfg.Mux.ClosePane,
		"broadcast":        cfg.Mux.Broadcast,
	}

	parsed := make(map[string]keyBinding, len(bindings))
	for name, value := range bindings {
		binding, err := parseKeyBinding(value)
		if err != nil {
			return nil, err
		}
		parsed[name] = binding
	}

	status := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true)
	status.SetBorder(true).SetTitle("lsmux")

	pages := tview.NewPages()
	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true).
		AddItem(status, 3, 0, false)

	m := &Manager{
		app:               app,
		conf:              cfg,
		names:             append([]string(nil), names...),
		command:           append([]string(nil), command...),
		initial:           append([]string(nil), initialHosts...),
		hold:              hold,
		allowLayoutChange: allowLayoutChange,
		factory:           NewSessionFactory(cfg, command),
		bindings:          parsed,
		root:              root,
		pages:             pages,
		status:            status,
		nextPageID:        1,
		nextPaneID:        1,
	}

	m.app.SetInputCapture(m.captureInput)
	m.app.SetMouseCapture(m.captureMouse)

	return m, nil
}

// Run starts the mux UI.
func (m *Manager) Run() error {
	if len(m.initial) > 0 {
		if err := m.createPage(m.initial); err != nil {
			return err
		}
		m.refreshMainPage()
	} else {
		m.showSelector(selectorInitial)
	}
	return m.app.SetRoot(m.root, true).EnableMouse(true).EnablePaste(true).Run()
}

func (m *Manager) captureMouse(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	if event == nil || m.currentPage == nil {
		return event, action
	}
	switch action {
	case tview.MouseScrollUp, tview.MouseScrollDown:
		x, y := event.Position()
		for _, p := range m.currentPage.panes {
			if p == nil || p.term == nil {
				continue
			}
			if !p.term.InRect(x, y) {
				continue
			}
			if action == tview.MouseScrollUp {
				p.term.ScrollbackUp(3)
			} else {
				p.term.ScrollbackDown(3)
			}
			if m.currentPage.focus == p {
				m.updateStatus("")
			}
			return nil, action
		}
	}
	return event, action
}

func (m *Manager) captureInput(event *tcell.EventKey) *tcell.EventKey {
	if event == nil {
		return nil
	}

	if m.selectorFocus != nil && m.app.GetFocus() == m.selectorFocus {
		return event
	}

	if event.Key() == tcell.KeyCtrlC {
		if m.broadcastAll {
			m.broadcastKey(event)
		}
		return tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone)
	}

	if event.Key() == tcell.KeyPgUp {
		m.scrollFocused(true)
		return nil
	}
	if event.Key() == tcell.KeyPgDn {
		m.scrollFocused(false)
		return nil
	}

	if m.bindings["prefix"].match(event) {
		m.prefixActive = true
		m.updateStatus("[gray]prefix[-]")
		return nil
	}

	if !m.prefixActive {
		if m.broadcastAll {
			m.broadcastKey(event)
		}
		return event
	}

	m.prefixActive = false
	switch {
	case m.bindings["new_page"].match(event):
		if !m.layoutChangeAllowed() {
			m.updateStatus("[red]layout change disabled in command mode[-]: use --allow-layout-change to enable")
			return nil
		}
		m.showSelector(selectorNewPage)
		return nil
	case m.bindings["new_pane"].match(event):
		if !m.layoutChangeAllowed() {
			m.updateStatus("[red]layout change disabled in command mode[-]: use --allow-layout-change to enable")
			return nil
		}
		m.showSelector(selectorNewPane)
		return nil
	case m.bindings["split_horizontal"].match(event):
		if !m.layoutChangeAllowed() {
			m.updateStatus("[red]layout change disabled in command mode[-]: use --allow-layout-change to enable")
			return nil
		}
		m.showSelector(selectorSplitHorizontal)
		return nil
	case m.bindings["split_vertical"].match(event):
		if !m.layoutChangeAllowed() {
			m.updateStatus("[red]layout change disabled in command mode[-]: use --allow-layout-change to enable")
			return nil
		}
		m.showSelector(selectorSplitVertical)
		return nil
	case m.bindings["next_pane"].match(event):
		m.cyclePane()
		return nil
	case m.bindings["next_page"].match(event):
		m.switchPage(m.findPageIndex(m.currentPage) + 1)
		return nil
	case m.bindings["prev_page"].match(event):
		m.switchPage(m.findPageIndex(m.currentPage) - 1)
		return nil
	case m.bindings["page_list"].match(event):
		m.showPageList()
		return nil
	case m.bindings["close_pane"].match(event):
		m.closeFocusedPane()
		return nil
	case m.bindings["quit"].match(event):
		m.app.Stop()
		return nil
	case m.bindings["broadcast"].match(event):
		m.broadcastAll = !m.broadcastAll
		m.refreshPaneStyles()
		m.updateStatus("")
		return nil
	default:
		return event
	}
}

func (m *Manager) layoutChangeAllowed() bool {
	return len(m.command) == 0 || m.allowLayoutChange
}

func (m *Manager) showSelector(mode selectorMode) {
	selector := list.NewTviewSelector(m.app, "lsmux>>", m.conf, m.names, true)
	selectorDirection := tview.FlexColumn
	if mode == selectorSplitHorizontal {
		selectorDirection = tview.FlexRow
	}
	if mode == selectorSplitVertical || mode == selectorNewPane {
		selectorDirection = tview.FlexColumn
	}

	var selectorPane *pane
	var previousFocus *pane
	var previousPage *page
	var selectorPage *page
	if mode == selectorNewPage {
		previousPage = m.currentPage
		selectorPane = m.newSelectorPane(selector)
		selectorPage = &page{
			name:  fmt.Sprintf("page-%d", m.nextPageID),
			panes: []*pane{selectorPane},
			focus: selectorPane,
			layout: &layoutNode{
				pane: selectorPane,
			},
		}
		m.nextPageID++
		m.sessionPages = append(m.sessionPages, selectorPage)
		m.currentPage = selectorPage
		m.refreshMainPage()
	}
	if mode == selectorNewPane || mode == selectorSplitHorizontal || mode == selectorSplitVertical {
		previousFocus = m.currentPage.focus
		selectorPane = m.newSelectorPane(selector)
		if err := m.insertTransientPane(selectorPane, selectorDirection); err != nil {
			m.updateStatus(fmt.Sprintf("[red]selector open failed[-]: %v", err))
			return
		}
		m.refreshMainPage()
	}

	selector.SetDoneFunc(func(hosts []string) {
		if len(hosts) == 0 {
			return
		}
		m.selectorFocus = nil
		m.setStatusVisible(true)
		if selectorPage != nil {
			m.removePage(selectorPage, previousPage)
		} else if selectorPane != nil {
			m.removeTransientPane(selectorPane, previousFocus)
		} else if mode == selectorInitial {
			m.pages.RemovePage("main")
		} else {
			m.pages.RemovePage("selector")
		}
		switch mode {
		case selectorInitial, selectorNewPage:
			if err := m.createPage(hosts); err != nil {
				m.updateStatus(fmt.Sprintf("[red]page open failed[-]: %v", err))
				return
			}
		case selectorNewPane:
			if err := m.addPanesToCurrentPage(hosts, tview.FlexColumn); err != nil {
				m.updateStatus(fmt.Sprintf("[red]pane open failed[-]: %v", err))
				return
			}
		case selectorSplitHorizontal:
			if err := m.addPanesToCurrentPage(hosts, tview.FlexRow); err != nil {
				m.updateStatus(fmt.Sprintf("[red]pane open failed[-]: %v", err))
				return
			}
		case selectorSplitVertical:
			if err := m.addPanesToCurrentPage(hosts, tview.FlexColumn); err != nil {
				m.updateStatus(fmt.Sprintf("[red]pane open failed[-]: %v", err))
				return
			}
		}
		if m.currentPage != nil {
			m.refreshMainPage()
		}
	})
	selector.SetCancelFunc(func() {
		m.selectorFocus = nil
		m.setStatusVisible(true)
		if selectorPage != nil {
			m.removePage(selectorPage, previousPage)
		} else if selectorPane != nil {
			m.removeTransientPane(selectorPane, previousFocus)
		} else if mode == selectorInitial {
			m.pages.RemovePage("main")
		} else {
			m.pages.RemovePage("selector")
		}
		if mode == selectorInitial && m.currentPage == nil {
			m.app.Stop()
			return
		}
		if m.currentPage != nil && m.currentPage.focus != nil {
			m.app.SetFocus(m.currentPage.focus.term)
		}
		m.updateStatus("")
	})

	if mode == selectorInitial {
		m.selectorFocus = selector.FocusTarget()
		m.setStatusVisible(true)
		m.pages.RemovePage("main")
		hostPane := tview.NewFlex().SetDirection(tview.FlexRow)
		hostPane.SetBorder(true)
		hostPane.SetTitle("select hosts")
		hostPane.AddItem(selector, 0, 1, true)
		m.pages.AddPage("main", hostPane, true, true)
		m.pages.SwitchToPage("main")
		m.app.SetFocus(selector.FocusTarget())
		return
	}

	if selectorPage != nil || selectorPane != nil {
		m.selectorFocus = selector.FocusTarget()
		m.app.SetFocus(selector.FocusTarget())
		m.updateStatus("[gray]select hosts[-]")
		return
	} else {
		m.selectorFocus = selector.FocusTarget()
		m.setStatusVisible(false)
		m.pages.RemovePage("selector")
		m.pages.AddPage("selector", selector, true, true)
		m.pages.SwitchToPage("selector")
	}
	m.app.SetFocus(selector.FocusTarget())
}

func (m *Manager) newSelectorPane(selector *list.TviewSelector) *pane {
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	container.SetBorder(true)
	container.SetTitle("select hosts")
	container.AddItem(selector, 0, 1, true)

	return &pane{
		id:          m.nextPaneID,
		server:      "selector",
		title:       "select hosts",
		primitive:   container,
		focusTarget: selector.FocusTarget(),
		transient:   true,
	}
}

func (m *Manager) insertTransientPane(p *pane, direction int) error {
	if m.currentPage == nil {
		return fmt.Errorf("no current page")
	}
	m.nextPaneID++
	if m.currentPage.layout == nil {
		m.currentPage.layout = &layoutNode{pane: p}
		m.currentPage.panes = append(m.currentPage.panes, p)
		m.currentPage.focus = p
		return nil
	}
	if !m.currentPage.layout.split(m.currentPage.focus, p, direction) {
		return fmt.Errorf("focused pane not found")
	}
	m.currentPage.panes = append(m.currentPage.panes, p)
	m.currentPage.focus = p
	return nil
}

func (m *Manager) removeTransientPane(target *pane, fallback *pane) {
	if m.currentPage == nil || target == nil {
		return
	}
	index := -1
	for i, p := range m.currentPage.panes {
		if p == target {
			index = i
			break
		}
	}
	if index >= 0 {
		m.currentPage.panes = append(m.currentPage.panes[:index], m.currentPage.panes[index+1:]...)
	}
	if m.currentPage.layout != nil {
		_ = m.currentPage.layout.remove(target)
	}
	if fallback != nil {
		m.currentPage.focus = fallback
	} else if len(m.currentPage.panes) > 0 {
		m.currentPage.focus = m.currentPage.panes[0]
	} else {
		m.currentPage.focus = nil
	}
}

func (m *Manager) removePage(target *page, fallback *page) {
	if target == nil {
		return
	}
	for i, candidate := range m.sessionPages {
		if candidate == target {
			m.sessionPages = append(m.sessionPages[:i], m.sessionPages[i+1:]...)
			break
		}
	}
	if fallback != nil {
		m.currentPage = fallback
	} else if len(m.sessionPages) > 0 {
		m.currentPage = m.sessionPages[0]
	} else {
		m.currentPage = nil
	}
}

func (m *Manager) setStatusVisible(visible bool) {
	if visible {
		m.root.ResizeItem(m.status, m.statusHeight(), 0)
		return
	}
	m.root.ResizeItem(m.status, 0, 0)
}

func (m *Manager) statusHeight() int {
	if m.prefixActive {
		return 6
	}
	return 3
}

func (m *Manager) createPage(hosts []string) error {
	page := &page{name: fmt.Sprintf("page-%d", m.nextPageID)}
	m.nextPageID++

	for _, host := range hosts {
		p := m.newPendingPane(host)
		page.panes = append(page.panes, p)
		m.startPaneConnect(page, p)
	}

	if len(page.panes) == 0 {
		return nil
	}

	page.focus = page.panes[0]
	if len(m.command) > 0 && len(page.panes) > 1 {
		page.layout = buildBalancedLayout(page.panes, tview.FlexColumn)
	} else {
		page.layout = &layoutNode{pane: page.panes[0]}
		for _, p := range page.panes[1:] {
			if !page.layout.split(page.focus, p, tview.FlexColumn) {
				return fmt.Errorf("failed to split pane layout")
			}
			page.focus = p
		}
	}

	m.sessionPages = append(m.sessionPages, page)
	m.currentPage = page
	return nil
}

func (m *Manager) addPanesToCurrentPage(hosts []string, direction int) error {
	if m.currentPage == nil {
		return m.createPage(hosts)
	}

	for _, host := range hosts {
		p := m.newPendingPane(host)
		if !m.currentPage.layout.split(m.currentPage.focus, p, direction) {
			if p.term != nil {
				_ = p.term.Close()
			}
			return fmt.Errorf("focused pane not found")
		}
		m.currentPage.panes = append(m.currentPage.panes, p)
		m.currentPage.focus = p
		m.startPaneConnect(m.currentPage, p)
	}
	return nil
}

func (m *Manager) newPendingPane(host string) *pane {
	body := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true)
	body.SetText("[yellow]connecting...[-]")

	container := tview.NewFlex().SetDirection(tview.FlexRow)
	container.SetBorder(true)
	container.AddItem(body, 0, 1, true)
	p := &pane{
		id:          m.nextPaneID,
		server:      host,
		title:       fmt.Sprintf("%s [connecting]", host),
		primitive:   container,
		focusTarget: body,
	}
	m.nextPaneID++
	container.SetTitle(p.title)
	return p
}

func (m *Manager) startPaneConnect(targetPage *page, p *pane) {
	if p == nil {
		return
	}
	host := p.server
	go func() {
		session, err := m.factory(host, 80, 24)
		m.app.QueueUpdateDraw(func() {
			if err != nil {
				m.replacePaneWithError(p, err)
				if m.currentPage == targetPage {
					m.refreshMainPage()
				}
				m.updateStatus(fmt.Sprintf("[red]%s connect failed[-]: %v", host, err))
				return
			}
			m.activatePane(targetPage, p, session)
			if m.currentPage == targetPage {
				m.refreshMainPage()
			}
			m.updateStatus(fmt.Sprintf("[green]%s connected[-]", host))
		})
	}()
}

func (m *Manager) activatePane(targetPage *page, p *pane, session *RemoteSession) {
	p.session = session
	p.term = tvxterm.New(m.app)
	p.primitive = nil
	p.focusTarget = nil
	p.failed = false
	p.exited = false
	p.exitMessage = ""
	p.title = m.paneTitle(p, "")
	p.term.SetBorder(true).SetTitle(p.title)
	p.term.SetTitleHandler(func(_ *tvxterm.View, title string) {
		m.app.QueueUpdateDraw(func() {
			p.title = m.paneTitle(p, title)
			p.term.SetTitle(p.title)
		})
	})
	p.term.SetBackendExitHandler(func(_ *tvxterm.View, err error) {
		m.app.QueueUpdateDraw(func() {
			if m.hold && len(m.command) > 0 {
				p.exited = true
				if err != nil {
					p.exitMessage = err.Error()
				} else {
					p.exitMessage = "completed"
				}
				p.title = m.paneTitle(p, "")
				p.term.SetTitle(p.title)
				m.updateStatus(fmt.Sprintf("[yellow]%s finished[-]: %s", p.server, p.exitMessage))
				return
			}
			m.removePane(targetPage, p)
			if err != nil {
				m.updateStatus(fmt.Sprintf("[red]%s closed[-]: %v", p.server, err))
			} else {
				m.updateStatus(fmt.Sprintf("[red]%s closed[-]", p.server))
			}
		})
	})
	p.term.Attach(session.Backend)
	m.applyPaneStyle(p)
}

func (m *Manager) replacePaneWithError(p *pane, err error) {
	if p == nil {
		return
	}
	body := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true)
	body.SetText(fmt.Sprintf("[red]connection failed[-]\n\n[white]%s[-]", tview.Escape(err.Error())))

	container := tview.NewFlex().SetDirection(tview.FlexRow)
	container.SetBorder(true)
	container.AddItem(body, 0, 1, true)

	p.term = nil
	p.session = nil
	p.title = fmt.Sprintf("%s [error]", p.server)
	p.primitive = container
	p.focusTarget = body
	p.failed = true
	p.exitMessage = err.Error()
	p.exited = false
	container.SetTitle(p.title)
}

func (m *Manager) newErrorPane(host string, err error) *pane {
	p := m.newPendingPane(host)
	m.replacePaneWithError(p, err)
	return p
}

func (m *Manager) paneTitle(p *pane, remoteTitle string) string {
	if remoteTitle != "" {
		return fmt.Sprintf("%s (%s)", p.server, remoteTitle)
	}
	if p.failed {
		return fmt.Sprintf("%s [error]", p.server)
	}
	if p.exited {
		return fmt.Sprintf("%s [done]", p.server)
	}
	if len(m.command) > 0 {
		return fmt.Sprintf("%s [cmd]", p.server)
	}
	return fmt.Sprintf("%s [shell]", p.server)
}

func (m *Manager) refreshMainPage() {
	m.pages.RemovePage("main")
	main := tview.NewFlex().SetDirection(tview.FlexRow)
	if m.currentPage != nil && m.currentPage.layout != nil {
		m.refreshPaneStyles()
		main.AddItem(m.currentPage.layout.primitive(), 0, 1, true)
	} else {
		main.AddItem(tview.NewBox(), 0, 1, true)
	}
	m.pages.AddPage("main", main, true, true)
	m.pages.SwitchToPage("main")
	if m.currentPage != nil && m.currentPage.focus != nil {
		m.app.SetFocus(m.currentPage.focus.focusPrimitive())
	}
	m.updateStatus("")
}

func (m *Manager) switchPage(index int) {
	if len(m.sessionPages) == 0 {
		return
	}
	if index < 0 {
		index = len(m.sessionPages) - 1
	}
	if index >= len(m.sessionPages) {
		index = 0
	}
	m.currentPage = m.sessionPages[index]
	m.refreshMainPage()
}

func (m *Manager) cyclePane() {
	if m.currentPage == nil || len(m.currentPage.panes) == 0 {
		return
	}
	current := 0
	for i, p := range m.currentPage.panes {
		if p == m.currentPage.focus {
			current = i
			break
		}
	}
	m.currentPage.focus = m.currentPage.panes[(current+1)%len(m.currentPage.panes)]
	m.refreshMainPage()
}

func (m *Manager) closeFocusedPane() {
	if m.currentPage == nil || m.currentPage.focus == nil {
		return
	}
	if m.currentPage.focus.failed {
		m.removePane(m.currentPage, m.currentPage.focus)
		return
	}
	if m.currentPage.focus.exited {
		m.removePane(m.currentPage, m.currentPage.focus)
		return
	}
	if m.currentPage.focus.term == nil {
		return
	}
	_ = m.currentPage.focus.term.Close()
}

func (m *Manager) removePane(targetPage *page, target *pane) {
	if targetPage == nil {
		return
	}
	index := -1
	for i, p := range targetPage.panes {
		if p == target {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}

	targetPage.panes = append(targetPage.panes[:index], targetPage.panes[index+1:]...)
	_ = targetPage.layout.remove(target)

	if len(targetPage.panes) == 0 {
		for i, candidate := range m.sessionPages {
			if candidate == targetPage {
				m.sessionPages = append(m.sessionPages[:i], m.sessionPages[i+1:]...)
				break
			}
		}
		if len(m.sessionPages) == 0 {
			m.currentPage = nil
			m.app.Stop()
			return
		}
		m.currentPage = m.sessionPages[0]
		m.refreshMainPage()
		return
	}

	if targetPage.focus == target {
		if index >= len(targetPage.panes) {
			index = len(targetPage.panes) - 1
		}
		targetPage.focus = targetPage.panes[index]
	}

	if m.currentPage == targetPage {
		m.refreshMainPage()
	}
}

func (m *Manager) showPageList() {
	if len(m.sessionPages) == 0 {
		return
	}

	view := tview.NewList().ShowSecondaryText(false)
	view.SetBorder(true).SetTitle("Pages")
	for i, p := range m.sessionPages {
		index := i
		view.AddItem(fmt.Sprintf("%s panes=%d", p.name, len(p.panes)), "", 0, func() {
			m.currentPage = m.sessionPages[index]
			m.pages.RemovePage("page-list")
			m.refreshMainPage()
		})
	}
	view.SetDoneFunc(func() {
		m.pages.RemovePage("page-list")
		if m.currentPage != nil && m.currentPage.focus != nil {
			m.app.SetFocus(m.currentPage.focus.focusPrimitive())
		}
		m.updateStatus("")
	})

	m.pages.RemovePage("page-list")
	m.pages.AddPage("page-list", centered(view, 60, minInt(len(m.sessionPages)+2, 14)), true, true)
	m.pages.SwitchToPage("page-list")
	m.app.SetFocus(view)
	m.updateStatus("[gray]page list[-]")
}

func (m *Manager) findPageIndex(target *page) int {
	for i, p := range m.sessionPages {
		if p == target {
			return i
		}
	}
	return 0
}

func (m *Manager) scrollFocused(up bool) {
	if m.currentPage == nil || m.currentPage.focus == nil || m.currentPage.focus.term == nil {
		return
	}
	if up {
		m.currentPage.focus.term.ScrollbackPageUp()
	} else {
		m.currentPage.focus.term.ScrollbackPageDown()
	}
	m.updateStatus("")
}

func (m *Manager) updateStatus(message string) {
	m.setStatusVisible(true)
	if m.currentPage == nil || m.currentPage.focus == nil {
		text := "[yellow]Select hosts to open panes[-]"
		if m.prefixActive {
			text += "  " + m.prefixHelp()
		} else {
			text += "  " + m.baseHelp()
		}
		m.status.SetText(text)
		return
	}
	if m.currentPage.focus.term == nil {
		text := fmt.Sprintf(
			"%s  [green]page[-]: %s  [green]pane[-]: %s",
			m.statusLead(),
			m.currentPage.name,
			m.currentPage.focus.server,
		)
		if m.currentPage.focus.failed && m.currentPage.focus.exitMessage != "" {
			text += "  [red]error[-]: " + tview.Escape(m.currentPage.focus.exitMessage)
		}
		if message != "" {
			text += "  " + message
		}
		m.status.SetText(text)
		return
	}

	offset, rows := m.currentPage.focus.term.ScrollbackStatus()
	broadcast := "off"
	if m.broadcastAll {
		broadcast = "on"
	}
	text := fmt.Sprintf(
		"%s  [green]page[-]: %s  [green]pane[-]: %s  [blue]scrollback[-]: %d/%d  [blue]pages[-]: %d  [blue]panes[-]: %d  [purple]broadcast[-]: %s",
		m.statusLead(),
		m.currentPage.name,
		m.currentPage.focus.server,
		offset,
		rows,
		len(m.sessionPages),
		len(m.currentPage.panes),
		broadcast,
	)
	if message != "" {
		text += "  " + message
	}
	if m.currentPage.focus.session != nil && m.currentPage.focus.session.LogPath != "" {
		text += "  [blue]log[-]: " + m.currentPage.focus.session.LogPath
	}
	m.status.SetText(text)
}

func (m *Manager) statusLead() string {
	if m.prefixActive {
		return m.prefixHelp()
	}
	return m.baseHelp()
}

func (m *Manager) baseHelp() string {
	return fmt.Sprintf("[yellow]Prefix[-]: %s", m.conf.Mux.Prefix)
}

func (m *Manager) prefixHelp() string {
	return fmt.Sprintf(
		"[yellow]Prefix[-]: %s  [yellow]new-page[-]: %s  [yellow]new-pane[-]: %s  [yellow]split-h[-]: %s  [yellow]split-v[-]: %s\n[yellow]next-pane[-]: %s  [yellow]next-page[-]: %s  [yellow]prev-page[-]: %s  [yellow]pages[-]: %s  [yellow]close[-]: %s  [yellow]broadcast[-]: %s  [yellow]quit[-]: %s",
		m.conf.Mux.Prefix,
		m.conf.Mux.NewPage,
		m.conf.Mux.NewPane,
		m.conf.Mux.SplitHorizontal,
		m.conf.Mux.SplitVertical,
		m.conf.Mux.NextPane,
		m.conf.Mux.NextPage,
		m.conf.Mux.PrevPage,
		m.conf.Mux.PageList,
		m.conf.Mux.ClosePane,
		m.conf.Mux.Broadcast,
		m.conf.Mux.Quit,
	)
}

func (m *Manager) broadcastKey(event *tcell.EventKey) {
	if event == nil || m.currentPage == nil || m.currentPage.focus == nil {
		return
	}
	for _, page := range m.sessionPages {
		for _, p := range page.panes {
			if p == nil || p.transient || p.term == nil || p == m.currentPage.focus {
				continue
			}
			_ = p.term.SendKey(cloneKeyEvent(event))
		}
	}
}

func (m *Manager) refreshPaneStyles() {
	for _, page := range m.sessionPages {
		for _, p := range page.panes {
			m.applyPaneStyle(p)
		}
	}
}

func (m *Manager) applyPaneStyle(p *pane) {
	if p == nil || p.term == nil {
		return
	}

	borderColor := tcell.ColorDefault
	titleColor := tcell.ColorDefault

	if m.broadcastAll && !p.transient {
		borderColor = parseMuxColor(m.conf.Mux.BroadcastBorderColor, borderColor)
		titleColor = parseMuxColor(m.conf.Mux.BroadcastTitleColor, titleColor)
	}
	if m.currentPage != nil && m.currentPage.focus == p {
		borderColor = parseMuxColor(m.conf.Mux.FocusBorderColor, borderColor)
		titleColor = parseMuxColor(m.conf.Mux.FocusTitleColor, titleColor)
	}

	p.term.SetBorderColor(borderColor)
	p.term.SetTitleColor(titleColor)
}

func parseMuxColor(value string, fallback tcell.Color) tcell.Color {
	if value == "" {
		return fallback
	}
	color := tcell.GetColor(value)
	if color == tcell.ColorDefault && value != "default" && value != "Default" {
		return fallback
	}
	return color
}

func cloneKeyEvent(event *tcell.EventKey) *tcell.EventKey {
	if event == nil {
		return nil
	}
	return tcell.NewEventKey(event.Key(), event.Rune(), event.Modifiers())
}

func (m *Manager) closeAll(panes []*pane) {
	for _, p := range panes {
		_ = p.term.Close()
	}
}

func centered(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().
				SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(p, height, 1, true).
				AddItem(nil, 0, 1, false),
			width, 1, true,
		).
		AddItem(nil, 0, 1, false)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
