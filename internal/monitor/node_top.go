// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"sync"

	mview "github.com/blacknon/mview"
)

var IOCount = 50

type NodeTop struct {
	Title         string
	Grid          *mview.Grid
	CPUUsage      *TopCPUUsage
	MemoryUsage   *TopMemoryUsage
	Uptimes       *TopUptime
	SystemSummary *TopSystemSummary
	DiskUsage     *TopDiskInfomation
	NetworkUsage  *TopNetworkInfomation

	sync.Mutex
}

func (t *NodeTop) focusPrimitives() []mview.Primitive {
	result := []mview.Primitive{}
	if t.DiskUsage != nil {
		result = append(result, t.DiskUsage)
	}
	if t.NetworkUsage != nil {
		result = append(result, t.NetworkUsage)
	}
	return result
}

func (t *NodeTop) firstFocusable() mview.Primitive {
	items := t.focusPrimitives()
	if len(items) == 0 {
		return nil
	}
	return items[0]
}

func (t *NodeTop) lastFocusable() mview.Primitive {
	items := t.focusPrimitives()
	if len(items) == 0 {
		return nil
	}
	return items[len(items)-1]
}

func (t *NodeTop) nextFocusable(current mview.Primitive) (mview.Primitive, bool) {
	items := t.focusPrimitives()
	for i, primitive := range items {
		if primitive == current {
			if i+1 < len(items) {
				return items[i+1], true
			}
			return nil, true
		}
	}
	return nil, false
}

func (t *NodeTop) prevFocusable(current mview.Primitive) (mview.Primitive, bool) {
	items := t.focusPrimitives()
	for i, primitive := range items {
		if primitive == current {
			if i-1 >= 0 {
				return items[i-1], true
			}
			return nil, true
		}
	}
	return nil, false
}

func (n *Node) CreateNodeTop() (err error) {
	// Top Image
	// | CPU()              | Memory()        | 2 line
	// |                    | Swap()          |
	// |                    | --------------- |
	// |                    | Kernel Version  | 4 line
	// |                    | Uptime          |
	// |                    | Tasks           |
	// |                    | LoadAvg         |
	// |                                      |
	// | ------------------------------------ |
	// | System Summary                       |
	// | ------------------------------------ |
	// | Disk                                 | 0(unlimited)
	// | Network                              | 0(unlimited)
	// Create PanelBaseTop
	top := &NodeTop{
		Title: fmt.Sprintf("TOP: %s", n.ServerName),
		Grid:  mview.NewGrid(),
	}

	// Set background color(no color)
	top.Grid.SetBackgroundColor(mview.ColorUnset)

	// Set columns
	top.Grid.SetColumns(50, 0, 0)

	top.resetWidgets(n)

	n.NodeTop = top

	return
}

func (t *NodeTop) resetWidgets(n *Node) {
	t.Lock()
	defer t.Unlock()

	t.Grid.Clear()

	t.CPUUsage = n.CreateTopCPUUsage()
	t.Uptimes = n.CreateTopUptime()
	t.MemoryUsage = n.CreateTopMemoryUsage()
	t.SystemSummary = n.CreateTopSystemSummary()
	t.DiskUsage = n.CreateTopDiskInfomation()
	t.NetworkUsage = n.CreateTopNetworkInfomation()

	t.updateGridRows(0, 0)
	t.applyLayout()
}

func (t *NodeTop) Refresh(n *Node) {
	if n == nil {
		return
	}

	if !n.CheckClientAlive() {
		t.resetWidgets(n)
		return
	}

	t.Lock()
	defer t.Unlock()

	wg := sync.WaitGroup{}

	wg.Add(6)
	t.CPUUsage.Update(&wg)
	t.MemoryUsage.Update(&wg)
	t.Uptimes.Update(&wg)
	t.SystemSummary.Update(&wg)
	t.DiskUsage.Update(&wg)
	t.NetworkUsage.Update(&wg)

	wg.Wait()

	diskRows := t.DiskUsage.GetRowCount()
	networkRows := t.NetworkUsage.GetRowCount()
	t.updateGridRows(diskRows, networkRows)
}

func (t *NodeTop) applyLayout() {
	t.Grid.Clear()

	t.Grid.AddItem(t.CPUUsage, 0, 0, 2, 1, 0, 0, true)
	t.Grid.AddItem(t.Uptimes, 0, 1, 1, 2, 0, 0, false)
	t.Grid.AddItem(t.MemoryUsage, 1, 1, 1, 2, 0, 0, false)

	t.Grid.AddItem(createEmptyPrimitive(), 2, 0, 1, 3, 0, 0, true)
	t.Grid.AddItem(t.SystemSummary, 3, 0, 1, 3, 0, 0, false)
	t.Grid.AddItem(createEmptyPrimitive(), 4, 0, 1, 3, 0, 0, true)
	t.Grid.AddItem(t.DiskUsage, 5, 0, 1, 3, 0, 0, false)
	t.Grid.AddItem(createEmptyPrimitive(), 6, 0, 1, 3, 0, 0, true)
	t.Grid.AddItem(t.NetworkUsage, 7, 0, 1, 3, 0, 0, false)
}

func (t *NodeTop) updateGridRows(diskRows, networkRows int) {
	summaryRows := 3
	if t.SystemSummary != nil && t.SystemSummary.GetRowCount() > 0 {
		summaryRows = t.SystemSummary.GetRowCount()
	}
	if diskRows <= 0 {
		diskRows = 1
	}
	if networkRows <= 0 {
		networkRows = 1
	}

	t.Grid.SetRows(4, 2, 1, summaryRows, 1, diskRows, 1, networkRows)
}

func (t *NodeTop) PreferredHeight() int {
	t.Lock()
	defer t.Unlock()

	summaryRows := 3
	if t.SystemSummary != nil && t.SystemSummary.GetRowCount() > 0 {
		summaryRows = t.SystemSummary.GetRowCount()
	}

	diskRows := 1
	if t.DiskUsage != nil {
		if rows := t.DiskUsage.GetRowCount(); rows > 1 {
			diskRows = rows
		}
	}

	networkRows := 1
	if t.NetworkUsage != nil {
		if rows := t.NetworkUsage.GetRowCount(); rows > 1 {
			networkRows = rows
		}
	}

	return 4 + 2 + 1 + summaryRows + 1 + diskRows + 1 + networkRows
}

func createEmptyPrimitive() mview.Primitive {
	empty := mview.NewTextView()
	empty.SetBackgroundColor(mview.ColorUnset)

	return empty
}
