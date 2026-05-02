// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"log"
	"runtime"
	"sync"
	"time"
	"unicode"

	"github.com/blacknon/lssh/internal/ssh"
	mview "github.com/blacknon/mview"
	"github.com/gdamore/tcell/v2"
)

// CreateBaseForm is create base list form.
func (m *Monitor) createBasePanel() (baseGrid *mview.Grid) {
	// create main tab table
	m.table = m.createBaseGridTable()
	m.topPane = newSwitchablePrimitive(nil)

	// 定期的なデータの更新を行わせる
	go m.updateBaseGridTableRows()
	go m.updateSelectedTopPanel()
	go m.updateCPUGraphTab()
	go m.reconnectServer()

	// create base grid
	baseGrid = mview.NewGrid()

	// Set title
	baseGrid.SetTitle("LIST")

	// Set background color(no color)
	baseGrid.SetBackgroundColor(mview.ColorUnset)

	// Set border options
	baseGrid.SetBorder(false)

	// Set columns
	baseGrid.SetColumns(0)

	// create rows
	baseGrid.SetRows(0, 1)

	footer := m.createFooter()
	baseGrid.AddItem(footer, 1, 0, 1, 3, 0, 0, false)

	// Layout for screens wider than 100 cells.
	baseGrid.AddItem(m.table, 0, 0, 1, 3, 0, 100, true)

	// Set input capture
	m.View.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if _, ok := m.View.GetFocus().(*tvxtermPrimitive); ok {
			if focus := m.View.GetFocus(); focus != nil {
				if handler := focus.InputHandler(); handler != nil {
					handler(event, func(p mview.Primitive) {
						m.View.SetFocus(p)
					})
				}
			}
			return nil
		}

		if m.handleGlobalMonitorKey(event) {
			return nil
		}

		if m.isTopEnabled() {
			top := m.getSelectedNodeTop()
			if top != nil {
				switch event.Key() {
				case tcell.KeyTab:
					if m.View.GetFocus() == m.table {
						if next := top.firstFocusable(); next != nil {
							m.View.SetFocus(next)
							return nil
						}
					} else if next, ok := top.nextFocusable(m.View.GetFocus()); ok {
						if next == nil {
							m.View.SetFocus(m.table)
						} else {
							m.View.SetFocus(next)
						}
						return nil
					}
				case tcell.KeyBacktab:
					if m.View.GetFocus() == m.table {
						if prev := top.lastFocusable(); prev != nil {
							m.View.SetFocus(prev)
							return nil
						}
					} else if prev, ok := top.prevFocusable(m.View.GetFocus()); ok {
						if prev == nil {
							m.View.SetFocus(m.table)
						} else {
							m.View.SetFocus(prev)
						}
						return nil
					}
				}
			}
		}

		return event
	})
	m.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return event
	})

	return
}

func (m *Monitor) handleGlobalMonitorKey(event *tcell.EventKey) bool {
	if event == nil {
		return false
	}

	if m.isExitConfirmVisible() {
		switch event.Key() {
		case tcell.KeyEscape:
			m.hideExitConfirm()
			return true
		case tcell.KeyCtrlC:
			return true
		}

		switch unicode.ToLower(event.Rune()) {
		case 'y':
			m.confirmExit()
			return true
		case 'n':
			m.hideExitConfirm()
			return true
		}

		return false
	}

	switch event.Key() {
	case tcell.KeyCtrlC:
		m.showExitConfirm()
		return true
	case tcell.KeyCtrlX:
		if !m.toggleTopEnabled() {
			m.View.SetFocus(m.table)
		}
		m.reDrawBasePanel()
		return true
	case tcell.KeyCtrlT:
		if m.isTopEnabled() && m.getSelectedNode() != "" {
			m.openTopTerminal()
			return true
		}
	}

	return false
}

func (m *Monitor) reconnectServer() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// reconnectServer owns the retry loop for nodes that have lost their
		// SSH/SFTP transport. Monitoring goroutines keep running and resume once
		// n.con becomes healthy again.
		var wg sync.WaitGroup
		for _, node := range m.Nodes {
			// Check client alive
			if !node.CheckClientAlive() {
				wg.Add(1)

				go func(n *Node, r *ssh.Run, wgg *sync.WaitGroup) {
					defer wgg.Done()
					log.Printf("try Reconnect Server: %s", n.ServerName)
					err := n.Connect(r)

					log.Printf("exit Reconnect Server: %s, err: %s", n.ServerName, err)
				}(node, m.r, &wg)
			}
		}

		wg.Wait()
		log.Printf("count runtime thread: %d \n", runtime.NumGoroutine())
		log.Printf("exit Reconnect loop\n")
	}
}

func (m *Monitor) reDrawBasePanel() {
	// baseGrid Clear
	m.BaseGrid.Clear()

	// BaseGridTop Clear

	// Set column
	m.BaseGrid.SetColumns(0)

	selectedNode := m.getSelectedNode()
	if m.isTopEnabled() && selectedNode != "" {
		m.BaseGrid.SetRows(6, 0, 1)

		m.syncTopPanelSelection()
		m.BaseGrid.AddItem(m.topPane, 1, 0, 1, 1, 0, 0, false)

		footer := m.createFooter()
		m.BaseGrid.AddItem(footer, 2, 0, 1, 1, 0, 0, false)

		// Layout for screens wider than 100 cells.
		m.BaseGrid.AddItem(m.table, 0, 0, 1, 1, 0, 100, true)
	} else {
		m.BaseGrid.SetRows(0, 1)

		footer := m.createFooter()
		m.BaseGrid.AddItem(footer, 1, 0, 1, 1, 0, 0, false)

		// Layout for screens wider than 100 cells.
		m.BaseGrid.AddItem(m.table, 0, 0, 1, 1, 0, 100, true)

		m.setTopEnabled(false)
	}

	m.syncTableSelectionToSelectedNode()

}

func (m *Monitor) syncTopPanelSelection() {
	top := m.getSelectedNodeTop()
	if top != nil {
		node := m.GetNode(m.getSelectedNode())
		top.Refresh(node)
	}
	m.refreshSelectedTopPanel()
}

func (m *Monitor) updateSelectedTopPanel() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if !m.isTopEnabled() {
			continue
		}

		if m.View == nil {
			continue
		}

		m.View.QueueUpdateDraw(func() {
			if !m.isTopEnabled() {
				return
			}

			server := m.getSelectedNode()
			if server == "" {
				return
			}

			node := m.GetNode(server)
			if node == nil || node.NodeTop == nil {
				return
			}

			node.NodeTop.Refresh(node)
		})
	}
}

func (m *Monitor) updateCPUGraphTab() {
	ticker := time.NewTicker(monitorSampleInterval)
	defer ticker.Stop()

	for range ticker.C {
		if m.View == nil || m.Panels == nil || m.cpuGraphTabName == "" {
			continue
		}

		if m.Panels.GetCurrentTab() != m.cpuGraphTabName {
			continue
		}

		m.View.QueueUpdateDraw(func() {})
	}
}

func (m *Monitor) getSelectedNodeTop() *NodeTop {
	selectedNode := m.getSelectedNode()
	if selectedNode == "" {
		return nil
	}

	node := m.GetNode(selectedNode)
	if node == nil {
		return nil
	}

	return node.NodeTop
}

func (m *Monitor) currentTopPrimitive() mview.Primitive {
	top := m.getSelectedNodeTop()
	if top == nil {
		return nil
	}

	base := newTopAlignedPrimitive(top.Title, top.Grid, top.PreferredHeight)

	if pane := m.getSelectedTopTerminal(); pane != nil {
		return newTopOverlayPrimitive(base, pane.Primitive, 60)
	}

	return base
}

func (m *Monitor) refreshSelectedTopPanel() {
	if m.topPane == nil {
		return
	}

	m.topPane.SetPrimitive(m.currentTopPrimitive())
}

func (m *Monitor) createFooter() mview.Primitive {
	footer := mview.NewTextView()

	footer.SetDynamicColors(true)
	footer.SetText("Ctrl-C[black:#00ffff]Quit[white:none] Ctrl-X[black:#00ffff]ToggleTopPanel[white:none] Ctrl-T[black:#00ffff]OpenTerminal[white]")
	footer.SetBackgroundColor(mview.ColorUnset)
	footer.SetTextAlign(mview.AlignLeft)

	return footer
}
