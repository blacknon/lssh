// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"sync"

	"github.com/blacknon/mview"
	"github.com/gdamore/tcell/v2"
)

type TopCPUUsage struct {
	*mview.Table

	Node *Node
}

func (n *Node) CreateTopCPUUsage() (result *TopCPUUsage) {
	// Create box
	table := mview.NewTable()

	// Set border options
	table.SetBorder(false)

	// Set background color(no color)
	table.SetBackgroundColor(mview.ColorUnset)

	// Set selected style
	table.SetSelectedStyle(tcell.ColorBlack, tcell.NewRGBColor(0, 255, 255), tcell.AttrNone)

	// Set fixed option
	table.SetFixed(1, 0)

	// Get and Set CPU Usage
	usages, err := n.GetCPUCoreUsage()
	if err != nil {
		return
	}

	// Headers
	headers := getTopCPUHeader()

	// Create rows
	rows := [][]string{}
	for i, _ := range usages {
		row := []string{}
		row = append(row, fmt.Sprintf("%6d", i))
		row = append(row, fmt.Sprintf("%8.1f%% [%-20s]", float64(0), "-"))

		rows = append(rows, row)
	}

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

	// Set table data
	for rowIndex, row := range rows {
		for colIndex, cell := range row {
			// TODO: col0の幅については不動のため、最大値を事前に取得して対応する
			tableCell := mview.NewTableCell(cell)
			tableCell.SetTextColor(tcell.ColorWhite)

			switch colIndex {
			case 1:
				tableCell.SetAlign(mview.AlignLeft)
			case 0:
				tableCell.SetAlign(mview.AlignRight)
			}

			table.SetCell(rowIndex+1, colIndex, tableCell)
		}
	}

	result = &TopCPUUsage{
		Table: table,
		Node:  n,
	}

	return result
}

func (t *TopCPUUsage) Update(wg *sync.WaitGroup) {
	defer wg.Done()

	if t.Node == nil {
		return
	}

	// Get and Set CPU Usage
	usages, err := t.Node.GetCPUCoreUsage()
	if err != nil {
		for i := 1; i <= t.Table.GetRowCount(); i++ {
			t.SetCell(i, 1, mview.NewTableCell(fmt.Sprintf("%6d", 0)))
			t.SetCell(i, 1, mview.NewTableCell(fmt.Sprintf("[gray]%8.1f%%[none][%-20s]", float64(0), "-")))
		}
	} else {
		// Set table data
		for rowIndex, usage := range usages {
			coreCell := mview.NewTableCell(fmt.Sprintf("%6d", rowIndex))
			coreCell.SetTextColor(tcell.NewRGBColor(0, 255, 255))

			barLength := 30
			lowBarLength := int(usage.Low * float64(barLength))
			normalBarLength := int(usage.Normal * float64(barLength))
			kernelBarLength := int(usage.Kernel * float64(barLength))
			guestBarLength := int(usage.Guest * float64(barLength))

			bar := ""

			for i := 0; i < lowBarLength; i++ {
				// lowBarLength
				bar += "[blue]|"
			}
			for i := lowBarLength; i < lowBarLength+normalBarLength; i++ {
				// normalBarLength
				bar += "[green]|"
			}
			for i := lowBarLength + normalBarLength; i < lowBarLength+normalBarLength+kernelBarLength; i++ {
				// kernelBarLength
				bar += "[red]|"
			}
			for i := lowBarLength + normalBarLength + kernelBarLength; i < lowBarLength+normalBarLength+kernelBarLength+guestBarLength; i++ {
				// guestBarLength
				bar += "[yellow]|"
			}
			for i := lowBarLength + normalBarLength + kernelBarLength + guestBarLength; i < barLength; i++ {
				// none
				bar += "[node] "
			}
			bar += "[none]"

			usageCell := mview.NewTableCell(fmt.Sprintf("[gray]%8.1f%%[none][%-20s]", usage.Total*100, bar))
			usageCell.SetTextColor(tcell.ColorWhite)

			t.SetCell(rowIndex+1, 0, coreCell)
			t.SetCell(rowIndex+1, 1, usageCell)
		}
	}

	sortColumn := t.GetSortClickedColumn()
	isDescending := t.GetSortClickedDescending()
	t.Sort(sortColumn, isDescending)
}

func getTopCPUHeader() []string {
	return []string{
		" Core",
		" Usage",
	}
}
