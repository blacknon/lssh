// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	mview "github.com/blacknon/mview"
	"sync"
)

type TopUptime struct {
	*mview.Table
	Node *Node
}

func (t *TopUptime) refreshHeaders() {
	if t == nil {
		return
	}

	headers := []string{" Kernel", " Uptime", " Tasks", " LoadAvg"}
	for row, header := range headers {
		t.Table.SetCell(row, 0, newMonitorHeaderCell(header))
	}
}

func (n *Node) CreateTopUptime() (result *TopUptime) {
	// Create box
	table := mview.NewTable()
	applyMonitorTableStyle(table, false)

	// Set Kernel
	KernelHeader := newMonitorHeaderCell(" Kernel")
	KernelValue := mview.NewTableCell("")
	table.SetCell(0, 0, KernelHeader)
	table.SetCell(0, 1, KernelValue)

	// Set Uptime
	UptimeHeader := newMonitorHeaderCell(" Uptime")
	UptimeValue := mview.NewTableCell("")
	table.SetCell(1, 0, UptimeHeader)
	table.SetCell(1, 1, UptimeValue)

	// Set Tasks
	TasksHeader := newMonitorHeaderCell(" Tasks")
	TasksValue := mview.NewTableCell("")
	table.SetCell(2, 0, TasksHeader)
	table.SetCell(2, 1, TasksValue)

	// Set LoadAvg
	LoadAvgHeader := newMonitorHeaderCell(" LoadAvg")
	LoadAvgValue := mview.NewTableCell("")
	table.SetCell(3, 0, LoadAvgHeader)
	table.SetCell(3, 1, LoadAvgValue)

	result = &TopUptime{
		Table: table,
		Node:  n,
	}
	result.refreshHeaders()

	return result
}

func (t *TopUptime) Update(wg *sync.WaitGroup) {
	defer wg.Done()
	if t.Node == nil {
		return
	}

	t.refreshHeaders()

	// Get and Set Kernel Version
	kernel, err := t.Node.GetKernelVersion()
	kernelText := "-"
	if err == nil {
		kernelText = kernel
	}
	kernelCell := mview.NewTableCell(fmt.Sprintf(" %s", kernelText))
	setMonitorAccentText(kernelCell)
	t.Table.SetCell(0, 1, kernelCell)

	// Get and Set Uptime
	uptime, err := t.Node.GetUptime()
	uptimeText := "-"
	if err == nil {
		uptimeText = uptimeFormatDuration(uptime.GetTotalDuration())
	}
	uptimeCell := mview.NewTableCell(fmt.Sprintf(" %s", uptimeText))
	setMonitorAccentText(uptimeCell)
	t.Table.SetCell(1, 1, uptimeCell)

	// Get and Set Tasks
	tasks, err := t.Node.GetTaskCounts()
	tasksText := "-"
	if err == nil {
		tasksText = fmt.Sprintf("%d", tasks)
	}
	tasksCell := mview.NewTableCell(fmt.Sprintf(" [gray]Tasks:[none] %s", tasksText))
	setMonitorMutedText(tasksCell)
	t.Table.SetCell(2, 1, tasksCell)

	// Get and Set LoadAvg
	loadavg, err := t.Node.GetLoadAvg()
	loadavgText := "-"
	if err == nil {
		loadavgText = fmt.Sprintf("[gray]1min:[none] %.2f, [gray]5min:[none] %.2f, [gray]15min:[none] %.2f", loadavg.Last1Min, loadavg.Last5Min, loadavg.Last15Min)
	}
	loadavgCell := mview.NewTableCell(fmt.Sprintf(" %s", loadavgText))
	setMonitorMutedText(loadavgCell)
	t.Table.SetCell(3, 1, loadavgCell)
}
