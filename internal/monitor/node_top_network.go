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
	"github.com/gdamore/tcell/v2"
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

	// Set border options
	table.SetBorder(false)

	// Set background color(no color)
	table.SetBackgroundColor(mview.ColorUnset)

	// Set selected style
	table.SetSelectedStyle(tcell.ColorBlack, tcell.NewRGBColor(0, 255, 255), tcell.AttrNone)

	// Set fixed option
	table.SetFixed(1, 0)

	// Headers
	headers := getTopNetworkHeader()

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
		tableCell.SetTextColor(tcell.NewRGBColor(0, 255, 255))
		t.SetCell(row, 0, tableCell)

		// IPv4Address
		ipv4 := networkUsage.IPv4Address
		tableCell = mview.NewTableCell(fmt.Sprintf("[gray]%s[none]", ipv4))
		tableCell.SetTextColor(tcell.ColorWhite)
		t.SetCell(row, 1, tableCell)

		// IPv6Address
		ipv6 := networkUsage.IPv6Address
		tableCell = mview.NewTableCell(fmt.Sprintf("[gray]%s[none]", ipv6))
		tableCell.SetTextColor(tcell.ColorWhite)
		t.SetCell(row, 2, tableCell)

		// RXBytes
		rxBytesLength := len(networkUsage.RXBytes)
		var networkRXBytesCell *mview.TableCell
		var rxBytes []float64
		if rxBytesLength > 0 {
			for i := 1; i < IOCount+1; i++ {
				var rxByte float64
				if i >= rxBytesLength {
					rxByte = float64(0)
				} else {
					rxByte = float64(networkUsage.RXBytes[rxBytesLength-i])
				}
				rxBytes = append(rxBytes, rxByte)
			}
			rxByte := humanize.Bytes(uint64(networkUsage.RXBytes[rxBytesLength-1]))
			maxBytes := scaleMaxValue(maxFloat64(rxBytes))
			readGraph := Graph{
				Data: rxBytes,
				Max:  maxBytes,
			}
			brailleLine := strings.Join(readGraph.BrailleLine(), "")

			networkRXBytes := fmt.Sprintf("[gray]%8s[none] [gray]%s[none]", rxByte, brailleLine)
			networkRXBytesCell = mview.NewTableCell(networkRXBytes)
		} else {
			networkRXBytes := fmt.Sprintf("[gray]%s[none]", "-")
			networkRXBytesCell = mview.NewTableCell(networkRXBytes)
		}
		networkRXBytesCell.SetTextColor(tcell.NewRGBColor(0, 255, 255))
		t.Table.SetCell(row, 3, networkRXBytesCell)

		// TXBytes
		txBytesLength := len(networkUsage.TXBytes)
		var networkTXBytesCell *mview.TableCell
		var txBytes []float64
		if txBytesLength > 0 {
			for i := 1; i < IOCount+1; i++ {
				var txByte float64
				if i >= txBytesLength {
					txByte = float64(0)
				} else {
					txByte = float64(networkUsage.TXBytes[txBytesLength-i])
				}
				txBytes = append(txBytes, txByte)
			}
			txByte := humanize.Bytes(uint64(networkUsage.TXBytes[txBytesLength-1]))
			maxBytes := scaleMaxValue(maxFloat64(txBytes))
			readGraph := Graph{
				Data: rxBytes,
				Max:  maxBytes,
			}
			brailleLine := strings.Join(readGraph.BrailleLine(), "")

			networkTXBytes := fmt.Sprintf("[gray]%8s[none] [gray]%s[none]", txByte, brailleLine)
			networkTXBytesCell = mview.NewTableCell(networkTXBytes)
		} else {
			networkTXBytes := fmt.Sprintf("[gray]%s[none]", "-")
			networkTXBytesCell = mview.NewTableCell(networkTXBytes)
		}
		networkTXBytesCell.SetTextColor(tcell.NewRGBColor(0, 255, 255))
		t.Table.SetCell(row, 4, networkTXBytesCell)

		// ReadPackets
		rxPacketLength := len(networkUsage.RXPackets)
		var networkRXPacketsCell *mview.TableCell
		var rxPackets []float64
		if rxPacketLength > 0 {
			for i := 1; i < IOCount+1; i++ {
				var rxPacket float64
				if i >= rxPacketLength {
					rxPacket = float64(0)
				} else {
					rxPacket = float64(networkUsage.RXPackets[rxPacketLength-i])
				}
				rxPackets = append(rxPackets, rxPacket)
			}
			rxPacket := networkUsage.RXPackets[rxPacketLength-1]
			maxPacket := scaleMaxValue(maxFloat64(rxPackets))
			readGraph := Graph{
				Data: rxPackets,
				Max:  maxPacket,
			}
			brailleLine := strings.Join(readGraph.BrailleLine(), "")

			networkRXPackets := fmt.Sprintf("[gray]%8d[none] [gray]%s[none]", rxPacket, brailleLine)
			networkRXPacketsCell = mview.NewTableCell(networkRXPackets)
		} else {
			networkRXPackets := fmt.Sprintf("[gray]%s[none]", "-")
			networkRXPacketsCell = mview.NewTableCell(networkRXPackets)
		}
		networkRXPacketsCell.SetTextColor(tcell.NewRGBColor(0, 255, 255))
		t.Table.SetCell(row, 5, networkRXPacketsCell)

		// writePackets
		txPacketLength := len(networkUsage.TXPackets)
		var networkTXPacketsCell *mview.TableCell
		var txPackets []float64
		if txPacketLength > 0 {
			for i := 1; i < IOCount+1; i++ {
				var txPacket float64
				if i >= txPacketLength {
					txPacket = float64(0)
				} else {
					txPacket = float64(networkUsage.TXPackets[txPacketLength-i])
				}
				txPackets = append(txPackets, txPacket)
			}
			txPacket := networkUsage.TXPackets[txPacketLength-1]
			maxPacket := scaleMaxValue(maxFloat64(txPackets))
			readGraph := Graph{
				Data: txPackets,
				Max:  maxPacket,
			}
			brailleLine := strings.Join(readGraph.BrailleLine(), "")

			networkTXPackets := fmt.Sprintf("[gray]%8d[none] [gray]%s[none]", txPacket, brailleLine)
			networkTXPacketsCell = mview.NewTableCell(networkTXPackets)
		} else {
			networkTXPackets := fmt.Sprintf("[gray]%s[none]", "-")
			networkTXPacketsCell = mview.NewTableCell(networkTXPackets)
		}
		networkTXPacketsCell.SetTextColor(tcell.NewRGBColor(0, 255, 255))
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
