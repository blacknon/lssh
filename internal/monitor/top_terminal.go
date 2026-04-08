package monitor

import (
	"fmt"
	"strings"
	"time"

	"github.com/blacknon/lssh/internal/mux"
	mview "github.com/blacknon/mview"
	"github.com/blacknon/tvxterm"
)

type topTerminalPane struct {
	Host      string
	Primitive mview.Primitive
	Session   *mux.RemoteSession
	Terminal  *tvxtermPrimitive
	stopDraw  chan struct{}
}

func (m *Monitor) openTopTerminal() {
	host := strings.TrimSpace(m.selectedNode)
	if !m.enableTop || host == "" {
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
			term.Attach(session.Backend)

			current.Session = session
			current.Terminal = newTVXTermPrimitive(term)
			current.Primitive = current.Terminal
			current.stopDraw = make(chan struct{})
			m.startTopTerminalDrawLoop(current)

			m.reDrawBasePanel()
			if m.selectedNode == server {
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

	if pane.stopDraw != nil {
		close(pane.stopDraw)
		pane.stopDraw = nil
	}

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
	host := strings.TrimSpace(m.selectedNode)
	if host == "" {
		return nil
	}
	return m.topTerminals[host]
}

func (m *Monitor) startTopTerminalDrawLoop(pane *topTerminalPane) {
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-pane.stopDraw:
				return
			case <-ticker.C:
				m.View.QueueUpdateDraw(func() {})
			}
		}
	}()
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
