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

// NOTE:
//   以下の内容を参考に、InterfaceとIPアドレスの対応を取得させる
//     https://stackoverflow.com/questions/5281341/get-local-network-interface-addresses-using-only-proc

type TopNetworkInfomation struct {
	*mview.Table
	Node *Node
}

func (n *Node) CreateTopNetworkInfomation() (result *TopNetworkInfomation) {
	// Create box
	table := mview.NewTable()
	applyMonitorTableStyle(table, true)

	// Allow row navigation when the top panel has focus.
	// Set fixed option
	table.SetFixed(1, 0)

	// Headers
	headers := getTopNetworkHeader()

	// Set table header
	for colIndex, header := range headers {
		table.SetCell(0, colIndex, newMonitorHeaderCell(header))
	}

	result = &TopNetworkInfomation{
		Table: table,
		Node:  n,
	}

	return result
}

func (t *TopNetworkInfomation) Update(wg *sync.WaitGroup) {
	defer wg.Done()
	if t.Node == nil {
		return
	}

	// Get Network Infomation
	networkUsages, err := t.Node.GetNetworkUsage()
	if err != nil {
		return
	}

	// Set Network Infomation
	for i, networkUsage := range networkUsages {
		row := i + 1

		// NetworkDevice
		device := networkUsage.Device
		tableCell := mview.NewTableCell(device)
		setMonitorAccentText(tableCell)
		t.SetCell(row, 0, tableCell)

		// IPv4Address
		ipv4 := networkUsage.IPv4Address
		tableCell = mview.NewTableCell(fmt.Sprintf("[gray]%s[none]", ipv4))
		tableCell.SetTextColor(monitorTextColor)
		t.SetCell(row, 1, tableCell)

		// IPv6Address
		ipv6 := networkUsage.IPv6Address
		tableCell = mview.NewTableCell(fmt.Sprintf("[gray]%s[none]", ipv6))
		tableCell.SetTextColor(monitorTextColor)
		t.SetCell(row, 2, tableCell)

		// RXBytes
		networkRXBytesCell := formatNetworkBytesCell(networkUsage.RXBytes, t.Node.GetNetworkGraphMaxBytes(networkUsage.Device))
		setMonitorAccentText(networkRXBytesCell)
		t.Table.SetCell(row, 3, networkRXBytesCell)

		// TXBytes
		networkTXBytesCell := formatNetworkBytesCell(networkUsage.TXBytes, t.Node.GetNetworkGraphMaxBytes(networkUsage.Device))
		setMonitorAccentText(networkTXBytesCell)
		t.Table.SetCell(row, 4, networkTXBytesCell)

		// ReadPackets
		networkRXPacketsCell := formatNetworkPacketsCell(networkUsage.RXPackets, t.Node.GetNetworkGraphMaxPackets(networkUsage.Device))
		setMonitorAccentText(networkRXPacketsCell)
		t.Table.SetCell(row, 5, networkRXPacketsCell)

		// writePackets
		networkTXPacketsCell := formatNetworkPacketsCell(networkUsage.TXPackets, t.Node.GetNetworkGraphMaxPackets(networkUsage.Device))
		setMonitorAccentText(networkTXPacketsCell)
		t.Table.SetCell(row, 6, networkTXPacketsCell)
	}

	sortColumn := t.GetSortClickedColumn()
	isDescending := t.GetSortClickedDescending()
	t.Sort(sortColumn, isDescending)
}

func getTopNetworkHeader() []string {
	return []string{
		" NetworkDevice",
		" IPv4Address",
		" IPv6Address",
		" RXBytes",
		" TXBytes",
		" RXPackets",
		" TXPackets",
	}
}

func formatNetworkBytesCell(history []uint64, graphMax uint64) *mview.TableCell {
	if len(history) == 0 {
		return mview.NewTableCell("[gray]-[none]")
	}

	values := networkHistorySeries(history)
	normalizedValues := normalizeFloat64SeriesToPercent(values, float64(graphMax))
	last := humanize.Bytes(history[len(history)-1])
	graph := Graph{Data: normalizedValues, Min: 0, Max: 100}
	brailleLine := strings.Join(graph.BrailleLine(), "")

	return mview.NewTableCell(fmt.Sprintf("[gray]%8s[none] [gray]%s[none]", last, brailleLine))
}

func formatNetworkPacketsCell(history []uint64, graphMax uint64) *mview.TableCell {
	if len(history) == 0 {
		return mview.NewTableCell("[gray]-[none]")
	}

	values := networkHistorySeries(history)
	normalizedValues := normalizeFloat64SeriesToPercent(values, float64(graphMax))
	last := history[len(history)-1]
	graph := Graph{Data: normalizedValues, Min: 0, Max: 100}
	brailleLine := strings.Join(graph.BrailleLine(), "")

	return mview.NewTableCell(fmt.Sprintf("[gray]%8d[none] [gray]%s[none]", last, brailleLine))
}

func networkHistorySeries(history []uint64) []float64 {
	length := len(history)
	result := make([]float64, 0, IOCount)
	for i := 1; i < IOCount+1; i++ {
		if i >= length {
			result = append(result, 0)
			continue
		}
		result = append(result, float64(history[length-i]))
	}
	return result
}
