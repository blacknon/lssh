// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/blacknon/lssh/internal/ssh"
	mview "github.com/blacknon/mview"
	"github.com/gdamore/tcell/v2"
)

// CreateBaseForm is create base list form.
func (m *Monitor) createBasePanel() (baseGrid *mview.Grid) {
	// create main tab table
	m.table = m.createBaseGridTable()

	// 定期的なデータの更新を行わせる
	go m.updateBaseGridTableRows()
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
	m.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlX:
			// toggle top panel
			m.enableTop = !m.enableTop

			// baseGrid Clear
			m.reDrawBasePanel()

			// draw
			m.View.Draw()
		}

		return event
	})

	return
}

func (m *Monitor) reconnectServer() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
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

	if m.enableTop && m.selectedNode != "" {
		m.BaseGrid.SetRows(6, 0, 1)

		// create top panel
		top := m.GetNode(m.selectedNode).NodeTop
		m.BaseGrid.AddItem(top.Grid, 1, 0, 1, 1, 0, 0, false)

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

		m.enableTop = false
	}

	// draw
	m.View.Draw()
}

func (m *Monitor) createFooter() mview.Primitive {
	footer := mview.NewTextView()

	footer.SetDynamicColors(true)
	footer.SetText("Ctrl-X[black:#00ffff]ToggleTopPanel[white]  ")
	footer.SetBackgroundColor(mview.ColorUnset)
	footer.SetTextAlign(mview.AlignLeft)

	return footer
}
