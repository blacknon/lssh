// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"strings"
	"sync"

	mview "github.com/blacknon/mview"
	"github.com/dustin/go-humanize"
)

type TopDiskInfomation struct {
	*mview.Table
	Node *Node
}

func (n *Node) CreateTopDiskInfomation() (result *TopDiskInfomation) {
	// Create box
	table := mview.NewTable()
	applyMonitorTableStyle(table, true)

	// Allow row navigation when the top panel has focus.
	// Set fixed option
	table.SetFixed(1, 0)

	// Headers
	headers := getTopDiskHeader()

	// Set table header
	for colIndex, header := range headers {
		table.SetCell(0, colIndex, newMonitorHeaderCell(header))
	}

	result = &TopDiskInfomation{
		Table: table,
		Node:  n,
	}

	return result
}

func (t *TopDiskInfomation) Update(wg *sync.WaitGroup) {
	defer wg.Done()
	if t.Node == nil {
		return
	}

	// Get and Set Memory Usage
	diksinfo, err := t.Node.GetDiskUsage()
	if err != nil {
		return
	}

	for i, disk := range diksinfo {
		row := i + 1

		// Disk Path(2)
		diskPathCell := mview.NewTableCell(disk.Device)
		setMonitorAccentText(diskPathCell)
		t.Table.SetCell(row, 0, diskPathCell)

		// Disk MountPoint(3)
		diskMountPointCell := mview.NewTableCell(fmt.Sprintf("[gray]%s[none]", disk.MountPoint))
		setMonitorAccentText(diskMountPointCell)
		t.Table.SetCell(row, 1, diskMountPointCell)

		// Disk Usage(Used/Total)(4)
		diskUsagePercent := float64(disk.Used) / float64(disk.All) * 100
		diskUsageBar := CreatePercentGraph(30, float64(disk.Used), float64(disk.All), "red")
		diskUsage := fmt.Sprintf("[yellow]%8.1f[gray]%%[none] [%s]", diskUsagePercent, diskUsageBar)
		diskUsageCell := mview.NewTableCell(diskUsage)
		setMonitorAccentText(diskUsageCell)
		t.Table.SetCell(row, 2, diskUsageCell)

		// Disk Use(Used)(5)
		diskUse := fmt.Sprintf("[gray]%8s[none]", humanize.Bytes(uint64(disk.Used)))
		diskUseCell := mview.NewTableCell(diskUse)
		setMonitorAccentText(diskUseCell)
		t.Table.SetCell(row, 3, diskUseCell)

		// Disk Total(6)
		diskTotal := fmt.Sprintf("[gray]%8s[none]", humanize.Bytes(uint64(disk.All)))
		diskTotalCell := mview.NewTableCell(diskTotal)
		setMonitorAccentText(diskTotalCell)
		t.Table.SetCell(row, 4, diskTotalCell)

		// Disk ReadIOBytes(7)
		readIOBytesLength := len(disk.ReadIOBytes)
		var diskReadIOCell *mview.TableCell
		var readBytes []float64
		if readIOBytesLength > 0 {
			for i := 1; i < IOCount+1; i++ {
				var readByte float64
				if i >= readIOBytesLength {
					readByte = float64(0)
				} else {
					readByte = float64(disk.ReadIOBytes[readIOBytesLength-i])
				}
				readBytes = append(readBytes, readByte)
			}
			readByte := humanize.Bytes(uint64(disk.ReadIOBytes[readIOBytesLength-1]))
			normalizedReadBytes := normalizeFloat64SeriesToPercent(
				readBytes,
				float64(t.Node.GetDiskReadGraphMaxBytes(disk.Device)),
			)
			readGraph := Graph{
				Data: normalizedReadBytes,
				Min:  0,
				Max:  100,
			}
			brailleLine := strings.Join(readGraph.BrailleLine(), "")

			diskReadIO := fmt.Sprintf("[gray]%8s[none] [gray]%s[none]", readByte, brailleLine)
			diskReadIOCell = mview.NewTableCell(diskReadIO)
		} else {
			diskReadIO := fmt.Sprintf("[gray]%s[none]", "-")
			diskReadIOCell = mview.NewTableCell(diskReadIO)
		}
		setMonitorAccentText(diskReadIOCell)
		t.Table.SetCell(row, 5, diskReadIOCell)

		// Disk WriteIOBytes(7)
		writeIOBytesLength := len(disk.WriteIOBytes)
		var diskWriteIOCell *mview.TableCell
		var writeBytes []float64
		if writeIOBytesLength > 0 {
			for i := 1; i < IOCount+1; i++ {
				var writeByte float64
				if i >= writeIOBytesLength {
					writeByte = float64(0)
				} else {
					writeByte = float64(disk.WriteIOBytes[writeIOBytesLength-i])
				}
				writeBytes = append(writeBytes, writeByte)
			}
			writeByte := humanize.Bytes(uint64(disk.WriteIOBytes[writeIOBytesLength-1]))
			normalizedWriteBytes := normalizeFloat64SeriesToPercent(
				writeBytes,
				float64(t.Node.GetDiskWriteGraphMaxBytes(disk.Device)),
			)
			readGraph := Graph{
				Data: normalizedWriteBytes,
				Min:  0,
				Max:  100,
			}
			brailleLine := strings.Join(readGraph.BrailleLine(), "")

			diskWriteIO := fmt.Sprintf("[gray]%8s[none] [gray]%s[none]", writeByte, brailleLine)
			diskWriteIOCell = mview.NewTableCell(diskWriteIO)
		} else {
			diskWriteIO := fmt.Sprintf("[gray]%s[none]", "-")
			diskWriteIOCell = mview.NewTableCell(diskWriteIO)
		}
		setMonitorAccentText(diskWriteIOCell)
		t.Table.SetCell(row, 6, diskWriteIOCell)
	}

	sortColumn := t.GetSortClickedColumn()
	isDescending := t.GetSortClickedDescending()
	t.Sort(sortColumn, isDescending)
}

func getTopDiskHeader() []string {
	return []string{
		" Disk",
		" MountPoint",
		" Usage",
		" Use",
		" Total",
		" ReadBytes",
		" WriteBytes",
	}
}
