// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"sync"
	"time"

	mview "github.com/blacknon/mview"
	"github.com/gdamore/tcell/v2"
)

var IOCount = 50

type NodeTop struct {
	Grid         *mview.Grid
	CPUUsage     *TopCPUUsage
	MemoryUsage  *TopMemoryUsage
	Uptimes      *TopUptime
	DiskUsage    *TopDiskInfomation
	NetworkUsage *TopNetworkInfomation

	sync.Mutex
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
	// | ------------------------------------ |
	// | Disk        | Process                | 0(unlimited)
	// | Network     |                        | 0(unlimited)
	// Create PanelBaseTop
	top := &NodeTop{
		Grid: mview.NewGrid(),
	}

	// Set title
	top.Grid.SetTitle(fmt.Sprintf("TOP: %s", n.ServerName))
	top.Grid.SetTitleAlign(mview.AlignLeft)
	top.Grid.SetTitleColor(tcell.NewRGBColor(0, 255, 255))

	// Set background color(no color)
	top.Grid.SetBackgroundColor(mview.ColorUnset)

	// Set border options
	top.Grid.SetBorder(true)
	top.Grid.SetBorderColor(tcell.ColorDarkGray)

	// Set columns
	top.Grid.SetColumns(50, 0, 0)

	// create rows
	top.Grid.SetRows(5, 2, 1, 0, 1, 0, -1)

	// create top panel
	top.CPUUsage = n.CreateTopCPUUsage()
	top.Uptimes = n.CreateTopUptime()
	top.MemoryUsage = n.CreateTopMemoryUsage()

	top.DiskUsage = n.CreateTopDiskInfomation()

	top.NetworkUsage = n.CreateTopNetworkInfomation()
	topProcess := n.createBaseGridTopProcess()

	// Add top panel
	// 1st, 2nd row
	top.Grid.AddItem(top.CPUUsage, 0, 0, 2, 1, 0, 0, true)
	top.Grid.AddItem(top.Uptimes, 0, 1, 1, 2, 0, 0, false)
	top.Grid.AddItem(top.MemoryUsage, 1, 1, 1, 2, 0, 0, false)

	// 3rd row
	top.Grid.AddItem(createEmptyPrimitive(), 2, 0, 1, 3, 0, 0, true)

	// 4th row
	top.Grid.AddItem(top.DiskUsage, 3, 0, 1, 3, 0, 0, false)

	// 5th row
	top.Grid.AddItem(createEmptyPrimitive(), 4, 0, 1, 3, 0, 0, true)

	// 6th row
	top.Grid.AddItem(top.NetworkUsage, 5, 0, 1, 3, 0, 0, false)

	// 7th row
	top.Grid.AddItem(topProcess, 6, 0, 1, 3, 0, 0, false)

	// go routine for update
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if !n.CheckClientAlive() {
				top.Grid.Clear()

				top.CPUUsage.Table.Clear()
				top.CPUUsage = n.CreateTopCPUUsage()

				top.MemoryUsage.Table.Clear()
				top.MemoryUsage = n.CreateTopMemoryUsage()

				top.Uptimes.Table.Clear()
				top.Uptimes = n.CreateTopUptime()

				top.DiskUsage.Table.Clear()
				top.DiskUsage = n.CreateTopDiskInfomation()

				top.NetworkUsage.Table.Clear()
				top.NetworkUsage = n.CreateTopNetworkInfomation()

				topProcess := n.createBaseGridTopProcess()

				// Add top panel
				// 1st, 2nd row
				top.Grid.AddItem(top.CPUUsage, 0, 0, 2, 1, 0, 0, true)
				top.Grid.AddItem(top.Uptimes, 0, 1, 1, 2, 0, 0, false)
				top.Grid.AddItem(top.MemoryUsage, 1, 1, 1, 2, 0, 0, false)

				// 3rd row
				top.Grid.AddItem(createEmptyPrimitive(), 2, 0, 1, 3, 0, 0, true)

				// 4th row
				top.Grid.AddItem(top.DiskUsage, 3, 0, 1, 3, 0, 0, false)

				// 5th row
				top.Grid.AddItem(createEmptyPrimitive(), 4, 0, 1, 3, 0, 0, true)

				// 6th row
				top.Grid.AddItem(top.NetworkUsage, 5, 0, 1, 3, 0, 0, false)

				// 7th row
				top.Grid.AddItem(topProcess, 6, 0, 1, 3, 0, 0, false)

				continue
			} else {
				wg := sync.WaitGroup{}

				wg.Add(5)
				top.CPUUsage.Update(&wg)
				top.MemoryUsage.Update(&wg)
				top.Uptimes.Update(&wg)
				top.DiskUsage.Update(&wg)
				top.NetworkUsage.Update(&wg)

				wg.Wait()
			}

			// Resize
			height4Row := top.DiskUsage.GetRowCount()
			height6Row := top.NetworkUsage.GetRowCount()

			top.Grid.SetRows(5, 2, 1, height4Row, 1, height6Row, -1)
		}
	}()

	n.NodeTop = top

	return
}

func (n *Node) createBaseGridTopProcess() mview.Primitive {
	topProcess := mview.NewTextView()
	topProcess.SetTextAlign(mview.AlignCenter)
	topProcess.SetText("")
	topProcess.SetBackgroundColor(mview.ColorUnset)

	return topProcess
}

func createEmptyPrimitive() mview.Primitive {
	empty := mview.NewTextView()
	empty.SetBackgroundColor(mview.ColorUnset)

	return empty
}
