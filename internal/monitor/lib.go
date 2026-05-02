// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"strings"
	"sync"

	"github.com/blacknon/lssh/internal/mux"
	sshrun "github.com/blacknon/lssh/internal/ssh"
	mview "github.com/blacknon/mview"
	"github.com/gdamore/tcell/v2"
)

type Monitor struct {
	// selected server list
	ServerList []string

	// sshrun.Run
	r *sshrun.Run

	// Node list
	Nodes []*Node

	// View
	View *mview.Application

	// Panel
	PanelCounter int
	Panels       *mview.TabbedPanels
	rootPanels   *mview.Panels

	// BaseTab(List)
	BaseGrid *mview.Grid
	// BaseTop      map[string]*NodeTop
	table              *mview.Table // MainTab(List)'s table
	top                *mview.Grid  // MainTab(List)'s top
	topPane            *switchablePrimitive
	selectedNode       string
	enableTop          bool // MainTab(List) enable Top
	syncingSelection   bool
	baseTableSortUsed  bool
	topTerminals       map[string]*topTerminalPane
	cpuGraphTabName    string
	cpuGraphColors     map[string]tcell.Color
	exitModal          *mview.Modal
	exitConfirmVisible bool
	exitConfirmFocus   mview.Primitive
	termFactory        mux.SessionFactory
	sharedTermFactory  mux.SessionFactory
	shareConnect       bool
	graphScale         GraphScaleConfig

	sync.Mutex
}

func (m *Monitor) getSelectedNode() string {
	m.Lock()
	defer m.Unlock()
	return strings.TrimSpace(m.selectedNode)
}

func (m *Monitor) setSelectedNode(server string) {
	m.Lock()
	m.selectedNode = strings.TrimSpace(server)
	m.Unlock()
}

func (m *Monitor) isTopEnabled() bool {
	m.Lock()
	defer m.Unlock()
	return m.enableTop
}

func (m *Monitor) setTopEnabled(enabled bool) {
	m.Lock()
	m.enableTop = enabled
	m.Unlock()
}

func (m *Monitor) toggleTopEnabled() bool {
	m.Lock()
	defer m.Unlock()
	m.enableTop = !m.enableTop
	return m.enableTop
}

func (m *Monitor) isSyncingSelection() bool {
	m.Lock()
	defer m.Unlock()
	return m.syncingSelection
}

func (m *Monitor) withSelectionSync(fn func()) {
	m.Lock()
	m.syncingSelection = true
	m.Unlock()

	defer func() {
		m.Lock()
		m.syncingSelection = false
		m.Unlock()
	}()

	if fn != nil {
		fn()
	}
}

func (m *Monitor) isBaseTableSortUsed() bool {
	m.Lock()
	defer m.Unlock()
	return m.baseTableSortUsed
}

func (m *Monitor) setBaseTableSortUsed(used bool) {
	m.Lock()
	m.baseTableSortUsed = used
	m.Unlock()
}

func (m *Monitor) isExitConfirmVisible() bool {
	m.Lock()
	defer m.Unlock()
	return m.exitConfirmVisible
}

func (m *Monitor) setExitConfirmVisible(visible bool, focus mview.Primitive) {
	m.Lock()
	m.exitConfirmVisible = visible
	if visible {
		m.exitConfirmFocus = focus
	} else {
		m.exitConfirmFocus = nil
	}
	m.Unlock()
}

func (m *Monitor) getExitConfirmFocus() mview.Primitive {
	m.Lock()
	defer m.Unlock()
	return m.exitConfirmFocus
}

func Run(r *sshrun.Run) (err error) {
	monitor := Monitor{}
	monitor.r = r

	monitor.enableTop = false
	monitor.topTerminals = map[string]*topTerminalPane{}
	monitor.termFactory = mux.NewSessionFactory(r.Conf, nil, mux.SessionOptions{
		IsBashrc:    r.IsBashrc,
		IsNotBashrc: r.IsNotBashrc,
	})
	monitor.sharedTermFactory = monitor.createSharedTopTerminalSession
	monitor.shareConnect = r.ShareConnect
	monitor.graphScale = newGraphScaleConfig(r.Conf.Monitor.Graph)
	monitor.cpuGraphColors = map[string]tcell.Color{}

	// Create WaitGroup
	wg := sync.WaitGroup{}

	// Create initial node objects and try the first SSH/SFTP connection.
	// A node may still start disconnected here. reconnectServer() owns the
	// retry loop and StartMonitoring() simply consumes the current connection.
	for _, server := range r.ServerList {
		wg.Add(1)
		go monitor.CreateNode(server, &wg)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	if len(monitor.Nodes) == 0 {
		err = fmt.Errorf("No server")
		return err
	}

	// Start Monitoring
	for i := range monitor.Nodes {
		go monitor.Nodes[i].StartMonitoring()
	}

	monitor.StartView()

	return err
}

func (m *Monitor) GetNode(server string) *Node {
	server = strings.TrimSpace(server)

	for i := range m.Nodes {
		if m.Nodes[i].ServerName == server {
			return m.Nodes[i]
		}
	}
	return nil
}

func (m *Monitor) CreateNode(server string, wg *sync.WaitGroup) {
	defer wg.Done()

	// CreateNode only prepares per-host state and the first connection.
	// Ongoing reconnect handling is delegated to reconnectServer().
	node := NewNode(server)
	node.ApplyGraphScaleConfig(m.graphScale)
	_ = node.Connect(m.r)

	m.Lock()
	m.Nodes = append(m.Nodes, node)
	m.Unlock()
}

func (m *Monitor) cpuGraphColorForServer(server string) tcell.Color {
	server = strings.TrimSpace(server)
	if server == "" {
		return cpuGraphPalette[0]
	}

	m.Lock()
	defer m.Unlock()

	if color, ok := m.cpuGraphColors[server]; ok {
		return color
	}

	usedColors := make(map[tcell.Color]struct{}, len(m.cpuGraphColors))
	for _, color := range m.cpuGraphColors {
		usedColors[color] = struct{}{}
	}

	for _, color := range cpuGraphPalette {
		if _, ok := usedColors[color]; ok {
			continue
		}
		m.cpuGraphColors[server] = color
		return color
	}

	color := cpuGraphPalette[len(m.cpuGraphColors)%len(cpuGraphPalette)]
	m.cpuGraphColors[server] = color
	return color
}
