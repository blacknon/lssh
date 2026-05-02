package monitor

import (
	"fmt"
	"strings"

	"github.com/blacknon/lssh/internal/mux"
	mview "github.com/blacknon/mview"
	"github.com/blacknon/tvxterm"
)

type topTerminalPane struct {
	Host      string
	Primitive mview.Primitive
	Session   *mux.RemoteSession
	Terminal  *tvxtermPrimitive
}

func (m *Monitor) openTopTerminal() {
	host := m.getSelectedNode()
	if !m.isTopEnabled() || host == "" {
		return
	}

	if pane := m.topTerminals[host]; pane != nil {
		m.reDrawBasePanel()
		if pane.Terminal != nil {
			m.View.SetFocus(pane.Terminal)
		}
		return
	}

	pane := &topTerminalPane{
		Host:      host,
		Primitive: m.createTopTerminalMessage(host, "connecting..."),
	}
	m.topTerminals[host] = pane
	m.reDrawBasePanel()

	go func(current *topTerminalPane, server string) {
		session, err := m.createTopTerminalSession(server, 80, 24)
		m.View.QueueUpdateDraw(func() {
			if m.topTerminals[server] != current {
				if session != nil && session.Backend != nil {
					_ = session.Backend.Close()
				}
				return
			}

			if err != nil {
				current.Primitive = m.createTopTerminalMessage(server, fmt.Sprintf("connect failed\n\n%s", err))
				m.reDrawBasePanel()
				m.View.SetFocus(m.table)
				return
			}

			term := tvxterm.New(nil)
			term.SetBorder(true)
			term.SetTitle(fmt.Sprintf("%s [shell]", server))
			term.SetTitleHandler(func(_ *tvxterm.View, title string) {
				m.View.QueueUpdateDraw(func() {
					if m.topTerminals[server] != current {
						return
					}
					if title == "" {
						term.SetTitle(fmt.Sprintf("%s [shell]", server))
					} else {
						term.SetTitle(fmt.Sprintf("%s (%s)", server, title))
					}
				})
			})
			term.SetBackendExitHandler(func(_ *tvxterm.View, err error) {
				m.View.QueueUpdateDraw(func() {
					if m.topTerminals[server] == current {
						m.closeTopTerminal(server, true)
						m.reDrawBasePanel()
					}
				})
			})
			backend := newRedrawNotifyingBackend(session.Backend, func() {
				if m.View != nil {
					m.View.QueueUpdateDraw(func() {})
				}
			})
			term.Attach(backend)

			current.Session = session
			current.Terminal = newTVXTermPrimitive(term)
			current.Primitive = current.Terminal

			m.reDrawBasePanel()
			if m.getSelectedNode() == server {
				m.View.SetFocus(current.Terminal)
			}
		})
	}(pane, host)
}

func (m *Monitor) closeTopTerminal(host string, refocusTable bool) {
	host = strings.TrimSpace(host)
	pane := m.topTerminals[host]
	if pane == nil {
		return
	}

	delete(m.topTerminals, host)

	switch {
	case pane.Terminal != nil:
		_ = pane.Terminal.Close()
	case pane.Session != nil && pane.Session.Backend != nil:
		_ = pane.Session.Backend.Close()
	}

	if refocusTable {
		m.View.SetFocus(m.table)
	}
}

func (m *Monitor) getSelectedTopTerminal() *topTerminalPane {
	host := m.getSelectedNode()
	if host == "" {
		return nil
	}
	return m.topTerminals[host]
}

func (m *Monitor) createTopTerminalSession(server string, cols, rows int) (*mux.RemoteSession, error) {
	if m.shareConnect {
		return m.sharedTermFactory(server, cols, rows)
	}
	return m.termFactory(server, cols, rows)
}

func (m *Monitor) createTopTerminalMessage(host string, message string) mview.Primitive {
	view := mview.NewTextView()
	view.SetBorder(true)
	view.SetTitle(fmt.Sprintf("%s [terminal]", host))
	view.SetDynamicColors(true)
	view.SetWrap(true)
	view.SetText(message)
	return view
}
