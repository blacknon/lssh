// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"log"

	mview "github.com/blacknon/mview"
)

// StartView is start view application.
func (m *Monitor) StartView() {
	// create app
	m.View = mview.NewApplication()
	defer m.View.HandlePanic()

	// enable mouse
	m.View.EnableMouse(true)

	// create Panels
	m.Panels = mview.NewTabbedPanels()
	m.PanelCounter = 0
	m.Panels.SetBackgroundColor(mview.ColorUnset)
	m.Panels.SetTabBackgroundColor(mview.ColorUnset)

	// Create base tab
	m.BaseGrid = m.createBasePanel()
	m.Panels.AddTab(fmt.Sprintf("panel-%d", m.PanelCounter), "Main", m.BaseGrid)
	m.PanelCounter++

	if graphTab := newCPUGraphTab(m); graphTab != nil {
		m.cpuGraphTabName = fmt.Sprintf("panel-%d", m.PanelCounter)
		m.Panels.AddTab(m.cpuGraphTabName, "Graph", graphTab)
		m.PanelCounter++
	}

	m.rootPanels = mview.NewPanels()
	m.exitModal = m.createExitConfirmModal()
	root := newExitOverlayPrimitive(m.Panels, m.exitModal, m.isExitConfirmVisible)

	m.View.SetRoot(root, true)
	m.View.SetFocus(m.table)

	// Start View app
	if err := m.View.Run(); err != nil {
		log.Printf("Error: %v\n", err)
	}
}

func (m *Monitor) DrawUpdate() {
	if m.View == nil {
		return
	}
	m.View.QueueUpdateDraw(func() {})
}

// CreateTab is create new Tab.
func (m *Monitor) createTab() {
	tabName := fmt.Sprintf("panel-%d", m.PanelCounter)
	labelName := "Tab"
	m.PanelCounter++

	form := mview.NewForm()
	m.Panels.AddTab(tabName, labelName, form)
}
