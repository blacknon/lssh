// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"bytes"
	"fmt"
	"sync"
	"time"
	"unsafe"

	mview "github.com/blacknon/mview"
	"github.com/gdamore/tcell/v2"
)

func (m *Monitor) createBaseGridTable() (table *mview.Table) {
	// create list object
	table = mview.NewTable()

	// Set table options
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)
	table.SetPadding(0, 0, 0, 3)
	table.SetSortClicked(true)

	// Set border options
	table.SetBorder(false)

	// Set background color(no color)
	table.SetBackgroundColor(mview.ColorUnset)

	// Set selected style
	table.SetSelectedStyle(tcell.ColorBlack, tcell.NewRGBColor(0, 255, 255), tcell.AttrNone)

	// Set selected func
	table.SetSelectionChangedFunc(
		func(row, col int) {
			if row > table.GetRowCount() || row < 0 {
				return
			}

			// Get server name
			cell := table.GetCell(row, 0)
			if isEmptyStruct(cell) {
				return
			}
			server := cell.GetText()

			// Get node
			node := m.GetNode(server)
			if node == nil {
				m.Lock()
				m.selectedNode = ""
				m.Unlock()
			} else {
				m.Lock()
				m.selectedNode = server
				m.Unlock()
			}

			if m.enableTop {
				m.reDrawBasePanel()
			}
		})

	// Headers
	headers := getServerHeader()

	// Rows
	rows := m.createBaseGridTableRows()

	// Set table header
	for colIndex, header := range headers {
		tableCell := mview.NewTableCell(header)
		tableCell.SetTextColor(tcell.ColorBlack)
		tableCell.SetBackgroundColor(tcell.ColorGreen)
		tableCell.SetAlign(mview.AlignLeft)
		tableCell.SetSelectable(false)
		tableCell.SetIsHeader(true)

		table.SetCell(0, colIndex, tableCell)
	}

	// get server name width
	serverNamewidth := 0
	for _, node := range m.Nodes {
		if len(node.ServerName) > serverNamewidth {
			serverNamewidth = len(node.ServerName)
		}
	}

	// Set table data
	for rowIndex, row := range rows {
		for colIndex, cell := range row {
			// TODO: col0の幅については不動のため、最大値を事前に取得して対応する
			if colIndex == 0 {
				template := fmt.Sprintf("%%-%ds", serverNamewidth)
				cell = fmt.Sprintf(template, cell)
			}
			tableCell := mview.NewTableCell(cell)

			tableCell.SetTextColor(tcell.ColorWhite)

			switch colIndex {
			case 1:
				tableCell.SetAlign(mview.AlignCenter)
			case 0:
				tableCell.SetAlign(mview.AlignLeft)
			case 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | 13:
				tableCell.SetAlign(mview.AlignRight)
			}

			table.SetCell(rowIndex+1, colIndex, tableCell)
		}
	}

	table.SetSortFunc(func(column int, i, j []byte) bool {
		switch column {
		case 5, 6, 7, 8:
			s1str := *(*string)(unsafe.Pointer(&i))
			s1int := parseSize(s1str)

			s2str := *(*string)(unsafe.Pointer(&j))
			s2int := parseSize(s2str)

			return s1int < s2int
		default:
			return bytes.Compare(i, j) == -1
		}
	})

	return table
}

func (m *Monitor) createBaseGridTableRows() (rows [][]string) {
	for _, node := range m.Nodes {
		row := []string{}

		// ServerName
		row = append(row, node.ServerName)

		for i := 0; i <= 13; i++ {
			row = append(row, "")
		}

		rows = append(rows, row)
	}

	return
}

type UpdateRows struct {
	data map[string][]*mview.TableCell
	sync.RWMutex
}

func (m *Monitor) updateBaseGridTableRows() {
	// Get table rows count
	count := m.table.GetRowCount()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		nodeRows := &UpdateRows{data: map[string][]*mview.TableCell{}}

		// Get update rows
		var wg1 sync.WaitGroup
		for i := 0; i < count-1; i++ {
			row := i + 1
			server := m.table.GetCell(row, 0).GetText()

			node := m.GetNode(server)
			if node == nil {
				continue
			}

			wg1.Add(1)
			go func() {
				defer wg1.Done()

				rows := m.updateBaseGridTableAtNode(node)

				nodeRows.Lock()
				nodeRows.data[server] = rows
				nodeRows.Unlock()
			}()
		}
		wg1.Wait()

		// Update rows
		var wg2 sync.WaitGroup
		for i := 0; i < count-1; i++ {
			row := i + 1
			server := m.table.GetCell(row, 0).GetText()

			if _, ok := nodeRows.data[server]; !ok {
				continue
			}

			for col, cell := range nodeRows.data[server] {
				wg2.Add(1)
				m.View.QueueUpdate(
					func() {
						defer wg2.Done()
						m.table.SetCell(row, col+1, cell)
					})
			}

		}
		wg2.Wait()

		sortColumn := m.table.GetSortClickedColumn()
		isDescending := m.table.GetSortClickedDescending()
		m.table.Sort(sortColumn, isDescending)

		m.View.Draw()
	}
}

func (m *Monitor) updateBaseGridTableAtNode(node *Node) (result []*mview.TableCell) {
	// if node is not connect, connect node.
	isConnect := node.CheckClientAlive()

	// 2nd(1)
	connectCell := m.getBaseGridTableDataIsConnect(isConnect)
	result = append(result, connectCell)

	// 3rd(2)
	uptimeCell := m.getBaseGridTableDataUptime(isConnect, node)
	result = append(result, uptimeCell)

	// 4th(3)
	cpuCoreCell := m.getBaseGridTableDataCPUCore(isConnect, node)
	result = append(result, cpuCoreCell)

	// 5th(4)
	// cpuUsageCell := m.getBaseGridTableDataCPUUsageWithSparkline(isConnect, node)
	cpuUsageCell := m.getBaseGridTableDataCPUUsageWithBrailleLine(isConnect, node)
	result = append(result, cpuUsageCell)

	// 6th, 7th, 8th, 9th(5, 6, 7, 8)
	memUseCell, memTotalCell, swapUseCell, swapTotalCell := m.getBaseGridTableDataMemUsage(isConnect, node)
	result = append(result, memUseCell, memTotalCell, swapUseCell, swapTotalCell)

	// 10th(9)
	tasksCell := m.getBaseGridTableDataTasks(isConnect, node)
	result = append(result, tasksCell)

	// 11th, 12th, 13th
	loadAvg15minCell, loadAvg5minCell, loadAvg1minCell := m.getBaseGridTableDataLoadAvg(isConnect, node)
	result = append(result, loadAvg15minCell, loadAvg5minCell, loadAvg1minCell)

	return
}

func getServerHeader() []string {
	return []string{
		" Server",
		" Connect",
		" Uptime",
		" Core",
		" CPU%",
		" MemUse",
		" MemTotal",
		" SwapUse",
		" SwapTotal",
		" Tasks",
		" LoadAvg15min",
		" LoadAvg5min",
		" LoadAvg1min",
	}
}
