// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package mux

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (m *Manager) captureMouse(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	if event == nil || m.currentPage == nil {
		return event, action
	}
	if action == tview.MouseLeftDown {
		x, y := event.Position()
		for _, p := range m.currentPage.panes {
			if p == nil || p.primitive == nil {
				continue
			}
			if tester, ok := p.primitive.(hitTester); ok && tester.hitTest(x, y) {
				if m.currentPage.focus != p {
					m.currentPage.focus = p
					m.refreshPaneStyles()
					m.updateStatus("")
				}
				if focus := p.focusPrimitive(); focus != nil {
					m.app.SetFocus(focus)
				}
				return event, action
			}
		}
		for _, p := range m.currentPage.panes {
			if p == nil || p.term == nil {
				continue
			}
			if !p.term.InRect(x, y) {
				continue
			}
			if m.currentPage.focus != p {
				m.currentPage.focus = p
				m.refreshPaneStyles()
				m.updateStatus("")
			}
			if focus := p.focusPrimitive(); focus != nil {
				m.app.SetFocus(focus)
			}
			return event, action
		}
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

	if m.selectorFocus != nil && (m.app.GetFocus() == m.selectorFocus || (m.currentPage != nil && m.currentPage.focus != nil && m.currentPage.focus.transient)) {
		if event.Key() == tcell.KeyCtrlC {
			return tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)
		}
		return event
	}
	if m.currentPage != nil && m.currentPage.focus != nil {
		focusTarget := m.currentPage.focus.focusPrimitive()
		if focusTarget != nil && m.app.GetFocus() == focusTarget && m.currentPage.focus.term != nil && focusTarget != m.currentPage.focus.term {
			if event.Key() == tcell.KeyCtrlC {
				return tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)
			}
			return event
		}
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
	case m.bindings["transfer"].match(event):
		m.showTransfer()
		return nil
	default:
		return event
	}
}

func (m *Manager) layoutChangeAllowed() bool {
	return len(m.command) == 0 || m.allowLayoutChange
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
	state := "running"
	if m.currentPage.focus.exited {
		state = "done"
	}
	text := fmt.Sprintf(
		"%s  [green]page[-]: %s  [green]pane[-]: %s  [blue]scrollback[-]: %d/%d  [blue]pages[-]: %d  [blue]panes[-]: %d  [purple]broadcast[-]: %s  [yellow]state[-]: %s",
		m.statusLead(),
		m.currentPage.name,
		m.currentPage.focus.server,
		offset,
		rows,
		len(m.sessionPages),
		len(m.currentPage.panes),
		broadcast,
		state,
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
	transferKey := m.conf.Mux.Transfer
	if !m.transferEnabled {
		transferKey = "disabled"
	}
	return fmt.Sprintf(
		"[yellow]Prefix[-]: %s  [yellow]new-page[-]: %s  [yellow]new-pane[-]: %s  [yellow]split-h[-]: %s  [yellow]split-v[-]: %s  [yellow]transfer[-]: %s\n[yellow]next-pane[-]: %s  [yellow]next-page[-]: %s  [yellow]prev-page[-]: %s  [yellow]pages[-]: %s  [yellow]close[-]: %s  [yellow]broadcast[-]: %s  [yellow]quit[-]: %s",
		m.conf.Mux.Prefix,
		m.conf.Mux.NewPage,
		m.conf.Mux.NewPane,
		m.conf.Mux.SplitHorizontal,
		m.conf.Mux.SplitVertical,
		transferKey,
		m.conf.Mux.NextPane,
		m.conf.Mux.NextPage,
		m.conf.Mux.PrevPage,
		m.conf.Mux.PageList,
		m.conf.Mux.ClosePane,
		m.conf.Mux.Broadcast,
		m.conf.Mux.Quit,
	)
}

func (m *Manager) showTransfer() {
	if !m.transferEnabled {
		m.updateStatus("[red]transfer unavailable[-]: disabled by config or option")
		return
	}
	if m.currentPage == nil || m.currentPage.focus == nil {
		m.updateStatus("[red]transfer unavailable[-]: no active pane")
		return
	}
	p := m.currentPage.focus
	if p.term == nil || p.session == nil || p.failed || p.exited || p.transient {
		m.updateStatus("[red]transfer unavailable[-]: select a connected pane")
		return
	}

	wizard := newTransferWizard(m, p)
	overlay := newModalOverlay(p.term, wizard.primitive())
	overlay.setFocusResolver(wizard.focusTarget)
	p.primitive = overlay
	p.focusTarget = nil
	m.refreshMainPage()
	if focus := p.focusPrimitive(); focus != nil {
		m.app.SetFocus(focus)
	}
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

	if p.exited {
		borderColor = parseMuxColor(m.conf.Mux.DoneBorderColor, borderColor)
		titleColor = parseMuxColor(m.conf.Mux.DoneTitleColor, titleColor)
		p.badgeColor = titleColor
	} else {
		p.badgeColor = tcell.ColorDefault
	}
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
	return newCenteredPrimitive(p, width, height)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
