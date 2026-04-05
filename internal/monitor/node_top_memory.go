// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"sync"

	mview "github.com/blacknon/mview"
	"github.com/c9s/goprocinfo/linux"
	"github.com/dustin/go-humanize"
	"github.com/gdamore/tcell/v2"
)

type TopMemoryUsage struct {
	*mview.Table
	Node *Node
}

func (n *Node) CreateTopMemoryUsage() (result *TopMemoryUsage) {
	// Create box
	table := mview.NewTable()

	// Set border options
	table.SetBorder(false)

	// Set background color(no color)
	table.SetBackgroundColor(mview.ColorUnset)

	// Set selected style
	table.SetSelectedStyle(tcell.ColorBlack, tcell.NewRGBColor(0, 255, 255), tcell.AttrNone)

	// Create Usage Size(memory)
	MemHeader := mview.NewTableCell(" Mem    ")
	MemHeader.SetTextColor(tcell.ColorBlack)
	MemHeader.SetBackgroundColor(tcell.ColorGreen)
	MemBar := mview.NewTableCell("")
	table.SetCell(0, 0, MemHeader)
	table.SetCell(0, 1, MemBar)

	// Swap
	SwapHeader := mview.NewTableCell(" Swp    ")
	SwapHeader.SetTextColor(tcell.ColorBlack)
	SwapHeader.SetBackgroundColor(tcell.ColorGreen)
	SwapBar := mview.NewTableCell("")
	table.SetCell(1, 0, SwapHeader)
	table.SetCell(1, 1, SwapBar)

	result = &TopMemoryUsage{
		Table: table,
		Node:  n,
	}

	return result
}

func (t *TopMemoryUsage) Update(wg *sync.WaitGroup) {
	defer wg.Done()
	if t.Node == nil {
		return
	}

	// Get and Set Memory Usage
	meminfo, err := t.Node.GetMemInfo()
	if err != nil {
		return
	}

	// Create Usage Size(memory)
	memUsed := (meminfo.MemTotal - meminfo.MemFree - meminfo.Buffers - meminfo.Cached) * 1024
	memTotal := (meminfo.MemTotal) * 1024
	humanizeMemUsed := humanize.Bytes(memUsed)
	humanizeMemTotal := humanize.Bytes(memTotal)

	// Create Usage Size(swap)
	swapUsed := (meminfo.SwapTotal - meminfo.SwapFree) * 1024
	swapTotal := (meminfo.SwapTotal) * 1024
	humanizeSwapUsed := humanize.Bytes(swapUsed)
	humanizeSwapTotal := humanize.Bytes(swapTotal)

	// Create Bar
	memBar, swapBar := CreateMemoryBarGraph(50, meminfo)

	// Memory
	MemBar := mview.NewTableCell(
		fmt.Sprintf(
			"[gray]%8s/%8s[none][%-30s]",
			humanizeMemUsed,
			humanizeMemTotal,
			memBar,
		),
	)
	t.Table.SetCell(0, 1, MemBar)

	// Swap
	SwapBar := mview.NewTableCell(
		fmt.Sprintf(
			"[gray]%8s/%8s[none][%-30s]",
			humanizeSwapUsed,
			humanizeSwapTotal,
			swapBar,
		),
	)
	t.Table.SetCell(1, 1, SwapBar)
}

func CreateMemoryBarGraph(length int, meminfo *linux.MemInfo) (memory, swap string) {
	// memory
	memUsedLength := int(float64(meminfo.MemTotal-meminfo.MemFree-meminfo.Buffers-meminfo.Cached) / float64(meminfo.MemTotal) * float64(length))
	memBufLength := int(float64(meminfo.Buffers) / float64(meminfo.MemTotal) * float64(length))
	memCacheLength := int(float64(meminfo.Cached) / float64(meminfo.MemTotal) * float64(length))

	// swap
	swapUsedLength := int(float64(meminfo.SwapTotal-meminfo.SwapFree) / float64(meminfo.SwapTotal) * float64(length))

	// Create Mem Bar
	memory = ""
	for i := 0; i < memUsedLength; i++ {
		// memUsedLength
		memory += "[green]|"
	}
	for i := memUsedLength; i < memUsedLength+memBufLength; i++ {
		// memBufLength
		memory += "[blue]|"
	}
	for i := memUsedLength + memBufLength; i < memUsedLength+memBufLength+memCacheLength; i++ {
		// memCacheLength
		memory += "[yellow]|"
	}
	for i := memUsedLength + memBufLength + memCacheLength; i < length; i++ {
		// none
		memory += "[node] "
	}
	memory += "[none]"

	// Create Swap Bar
	swap = ""
	for i := 0; i < swapUsedLength; i++ {
		// swapUsedLength
		swap += "[red]|"
	}
	for i := swapUsedLength; i < length; i++ {
		// none
		swap += "[node] "
	}
	swap += "[none]"

	return memory, swap
}
