// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"sync"

	mview "github.com/blacknon/mview"
	"github.com/gdamore/tcell/v2"
)

type TopUptime struct {
	*mview.Table
	Node *Node
}

func (n *Node) CreateTopUptime() (result *TopUptime) {
	// Create box
	table := mview.NewTable()

	// Set border options
	table.SetBorder(false)

	// Set background color(no color)
	table.SetBackgroundColor(mview.ColorUnset)

	// Set selected style
	table.SetSelectedStyle(tcell.ColorBlack, tcell.NewRGBColor(0, 255, 255), tcell.AttrNone)

	// Set Kernel
	KernelHeader := mview.NewTableCell(" Kernel")
	KernelHeader.SetTextColor(tcell.ColorBlack)
	KernelHeader.SetBackgroundColor(tcell.ColorGreen)
	KernelValue := mview.NewTableCell("")
	table.SetCell(0, 0, KernelHeader)
	table.SetCell(0, 1, KernelValue)

	// Set Uptime
	UptimeHeader := mview.NewTableCell(" Uptime")
	UptimeHeader.SetTextColor(tcell.ColorBlack)
	UptimeHeader.SetBackgroundColor(tcell.ColorGreen)
	UptimeValue := mview.NewTableCell("")
	table.SetCell(1, 0, UptimeHeader)
	table.SetCell(1, 1, UptimeValue)

	// Set Tasks
	TasksHeader := mview.NewTableCell(" Tasks")
	TasksHeader.SetTextColor(tcell.ColorBlack)
	TasksHeader.SetBackgroundColor(tcell.ColorGreen)
	TasksValue := mview.NewTableCell("")
	table.SetCell(2, 0, TasksHeader)
	table.SetCell(2, 1, TasksValue)

	// Set LoadAvg
	LoadAvgHeader := mview.NewTableCell(" LoadAvg")
	LoadAvgHeader.SetTextColor(tcell.ColorBlack)
	LoadAvgHeader.SetBackgroundColor(tcell.ColorGreen)
	LoadAvgValue := mview.NewTableCell("")
	table.SetCell(3, 0, LoadAvgHeader)
	table.SetCell(3, 1, LoadAvgValue)

	result = &TopUptime{
		Table: table,
		Node:  n,
	}

	return result
}

func (t *TopUptime) Update(wg *sync.WaitGroup) {
	defer wg.Done()
	if t.Node == nil {
		return
	}

	// Get and Set Kernel Version
	kernel, err := t.Node.GetKernelVersion()
	if err != nil {
		return
	}
	kernelCell := mview.NewTableCell(fmt.Sprintf(" %s", kernel))
	kernelCell.SetTextColor(tcell.NewRGBColor(0, 255, 255))
	t.Table.SetCell(0, 1, kernelCell)

	// Get and Set Uptime
	uptime, err := t.Node.GetUptime()
	if err != nil {
		return
	}
	uptimeTotal := uptime.GetTotalDuration()
	uptimeCell := mview.NewTableCell(fmt.Sprintf(" %s", uptimeFormatDuration(uptimeTotal)))
	uptimeCell.SetTextColor(tcell.NewRGBColor(0, 255, 255))
	t.Table.SetCell(1, 1, uptimeCell)

	// Get and Set Tasks
	tasks, err := t.Node.GetTaskCounts()
	if err != nil {
		return
	}
	tasksCell := mview.NewTableCell(fmt.Sprintf(" [gray]Tasks:[none] %d", tasks))
	tasksCell.SetTextColor(tcell.ColorGray)
	t.Table.SetCell(2, 1, tasksCell)

	// Get and Set LoadAvg
	loadavg, err := t.Node.GetLoadAvg()
	if err != nil {
		return
	}
	loadAvg1min := loadavg.Last1Min
	loadAvg5min := loadavg.Last5Min
	loadAvg15min := loadavg.Last15Min

	loadavgCell := mview.NewTableCell(fmt.Sprintf(" [gray]1min:[none] %.2f, [gray]5min:[none] %.2f, [gray]15min:[none] %.2f", loadAvg1min, loadAvg5min, loadAvg15min))
	loadavgCell.SetTextColor(tcell.ColorGray)
	t.Table.SetCell(3, 1, loadavgCell)
}
