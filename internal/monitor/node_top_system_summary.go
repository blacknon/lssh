// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	mview "github.com/blacknon/mview"
	"github.com/mattn/go-runewidth"
)

const (
	systemSummaryPressureCellWidth = 24
	systemSummarySocketCellWidth   = 18
)

var systemSummaryStyleTagPattern = regexp.MustCompile(`\[[^\]]*\]`)

type TopSystemSummary struct {
	*mview.Table
	Node *Node
}

type systemSummaryRow struct {
	label string
	pairs []systemSummaryPair
}

type systemSummaryPair struct {
	header string
	value  string
}

func (n *Node) CreateTopSystemSummary() *TopSystemSummary {
	table := mview.NewTable()
	applyMonitorTableStyle(table, false)

	result := &TopSystemSummary{
		Table: table,
		Node:  n,
	}
	result.renderRows([]systemSummaryRow{
		{label: " CPU PSI"},
		{label: " Mem/IO"},
		{label: " Socket"},
	})

	return result
}

func (t *TopSystemSummary) Update(wg *sync.WaitGroup) {
	defer wg.Done()
	if t.Node == nil {
		return
	}

	t.renderRows(buildSystemSummaryRows(t.Node))
}

func (t *TopSystemSummary) renderRows(rows []systemSummaryRow) {
	t.Table.Clear()

	for rowIndex, row := range rows {
		header := mview.NewTableCell(row.label)
		header.SetTextColor(monitorBaseColor)
		header.SetBackgroundColor(monitorAccentColor)
		header.SetAlign(mview.AlignLeft)
		t.Table.SetCell(rowIndex, 0, header)

		for pairIndex, pair := range row.pairs {
			headerCol := 1 + pairIndex*2
			valueCol := headerCol + 1

			pairHeader := newMonitorHeaderCell(pair.header)
			t.Table.SetCell(rowIndex, headerCol, pairHeader)

			pairValue := mview.NewTableCell(pair.value)
			pairValue.SetTextColor(monitorTextColor)
			pairValue.SetAlign(mview.AlignLeft)
			t.Table.SetCell(rowIndex, valueCol, pairValue)
		}
	}
}

func buildSystemSummaryRows(node *Node) []systemSummaryRow {
	return []systemSummaryRow{
		buildCPUPressureRow(node),
		buildMemIORow(node),
		buildSocketRow(node),
	}
}

func buildCPUPressureRow(node *Node) systemSummaryRow {
	row := systemSummaryRow{
		label: " CPU PSI",
		pairs: []systemSummaryPair{
			{header: " some10", value: "-"},
			{header: " some60", value: "-"},
			{header: " some300", value: "-"},
		},
	}

	cpuPressure, err := node.GetPressureCPU()
	if err != nil || cpuPressure == nil || cpuPressure.Some == nil {
		return row
	}

	some10History, some60History, some300History, historyErr := node.GetCPUPressureHistory()
	row.pairs[0].value = formatPressureCell(cpuPressure.Some.Avg10, some10History, 12)
	row.pairs[1].value = formatPressureCell(cpuPressure.Some.Avg60, some60History, 12)
	row.pairs[2].value = formatPressureCell(cpuPressure.Some.Avg300, some300History, 12)
	if historyErr != nil {
		row.pairs[0].value = formatPressureCellNoHistory(cpuPressure.Some.Avg10)
		row.pairs[1].value = formatPressureCellNoHistory(cpuPressure.Some.Avg60)
		row.pairs[2].value = formatPressureCellNoHistory(cpuPressure.Some.Avg300)
	}

	return row
}

func buildMemIORow(node *Node) systemSummaryRow {
	row := systemSummaryRow{
		label: " Mem/IO",
		pairs: []systemSummaryPair{
			{header: " mem10", value: "-"},
			{header: " mem60", value: "-"},
			{header: " io10", value: "-"},
			{header: " io60", value: "-"},
		},
	}

	memCurrent, memErr := node.GetPressureMem()
	ioCurrent, ioErr := node.GetPressureIO()
	if memErr != nil && ioErr != nil {
		return row
	}

	mem10History, mem60History, memHistoryErr := node.GetMemPressureHistory()
	io10History, io60History, ioHistoryErr := node.GetIOPressureHistory()

	if memErr == nil && memCurrent != nil && memCurrent.Some != nil {
		row.pairs[0].value = formatPressureCell(memCurrent.Some.Avg10, mem10History, 10)
		row.pairs[1].value = formatPressureCell(memCurrent.Some.Avg60, mem60History, 10)
		if memHistoryErr != nil {
			row.pairs[0].value = formatPressureCellNoHistory(memCurrent.Some.Avg10)
			row.pairs[1].value = formatPressureCellNoHistory(memCurrent.Some.Avg60)
		}
	}
	if ioErr == nil && ioCurrent != nil && ioCurrent.Some != nil {
		row.pairs[2].value = formatPressureCell(ioCurrent.Some.Avg10, io10History, 10)
		row.pairs[3].value = formatPressureCell(ioCurrent.Some.Avg60, io60History, 10)
		if ioHistoryErr != nil {
			row.pairs[2].value = formatPressureCellNoHistory(ioCurrent.Some.Avg10)
			row.pairs[3].value = formatPressureCellNoHistory(ioCurrent.Some.Avg60)
		}
	}

	return row
}

func buildSocketRow(node *Node) systemSummaryRow {
	row := systemSummaryRow{
		label: " Socket",
		pairs: []systemSummaryPair{
			{header: " fd", value: "-"},
			{header: " est", value: "-"},
			{header: " tw", value: "-"},
			{header: " lis", value: "-"},
		},
	}

	fileNr, fileErr := node.GetFileNr()
	tcpStates, tcpErr := node.GetTCPStates()
	fdHistory, fdHistoryErr := node.GetFileNrHistory()
	estHistory, twHistory, listenHistory, tcpHistoryErr := node.GetTCPStateHistory()
	if fileErr != nil && tcpErr != nil {
		return row
	}

	if fileErr == nil && fileNr != nil {
		row.pairs[0].value = formatUintCell(fileNr.Allocated, fdHistory, 10, systemSummarySocketCellWidth, node.GetFDGraphMax())
		if fdHistoryErr != nil {
			row.pairs[0].value = formatUintCellNoHistory(fileNr.Allocated, systemSummarySocketCellWidth)
		}
	}
	if tcpErr == nil {
		row.pairs[1].value = formatUintCell(tcpStates["ESTABLISHED"], estHistory, 8, systemSummarySocketCellWidth, 0)
		row.pairs[2].value = formatUintCell(tcpStates["TIME_WAIT"], twHistory, 8, systemSummarySocketCellWidth, 0)
		row.pairs[3].value = formatUintCell(tcpStates["LISTEN"], listenHistory, 8, systemSummarySocketCellWidth, 0)
		if tcpHistoryErr != nil {
			row.pairs[1].value = formatUintCellNoHistory(tcpStates["ESTABLISHED"], systemSummarySocketCellWidth)
			row.pairs[2].value = formatUintCellNoHistory(tcpStates["TIME_WAIT"], systemSummarySocketCellWidth)
			row.pairs[3].value = formatUintCellNoHistory(tcpStates["LISTEN"], systemSummarySocketCellWidth)
		}
	}

	return row
}

func historySparklineFloat64(values []float64, width int, min, max float64) string {
	trimmed := trimFloat64History(values, width)
	if len(trimmed) == 0 {
		return ""
	}

	graph := Graph{
		Data: trimmed,
		Min:  min,
		Max:  max,
	}
	return strings.Join(graph.BrailleLine(), "")
}

func historySparklineUint64(values []uint64, width int, graphMax uint64) string {
	trimmed := trimUint64History(values, width)
	if len(trimmed) == 0 {
		return ""
	}

	maxValue := float64(graphMax)
	if maxValue == 0 {
		for _, value := range trimmed {
			if float64(value) > maxValue {
				maxValue = float64(value)
			}
		}
	}
	if maxValue == 0 {
		maxValue = 1
	}

	data := make([]float64, 0, len(trimmed))
	for _, value := range trimmed {
		data = append(data, float64(value)/maxValue*100)
	}

	graph := Graph{
		Data: data,
		Min:  0,
		Max:  100,
	}
	return strings.Join(graph.BrailleLine(), "")
}

func trimFloat64History(values []float64, width int) []float64 {
	if len(values) == 0 {
		return nil
	}
	if width <= 0 || len(values) <= width {
		return append([]float64(nil), values...)
	}
	return append([]float64(nil), values[len(values)-width:]...)
}

func trimUint64History(values []uint64, width int) []uint64 {
	if len(values) == 0 {
		return nil
	}
	if width <= 0 || len(values) <= width {
		return append([]uint64(nil), values...)
	}
	return append([]uint64(nil), values[len(values)-width:]...)
}

func formatPressureCell(value float64, history []float64, graphWidth int) string {
	graph := historySparklineFloat64(history, graphWidth, 0, 100)
	if graph == "" {
		return formatPressureCellNoHistory(value)
	}
	text := fmt.Sprintf("[gray]%6.2f[none] [gray]%-*s[none]", value, graphWidth, graph)
	return padDisplayText(text, systemSummaryPressureCellWidth)
}

func formatPressureCellNoHistory(value float64) string {
	return padDisplayText(fmt.Sprintf("[gray]%6.2f[none]", value), systemSummaryPressureCellWidth)
}

func formatUintCell(value uint64, history []uint64, graphWidth int, totalWidth int, graphMax uint64) string {
	graph := historySparklineUint64(history, graphWidth, graphMax)
	if graph == "" {
		return formatUintCellNoHistory(value, totalWidth)
	}
	text := fmt.Sprintf("[gray]%6d[none] [gray]%-*s[none]", value, graphWidth, graph)
	return padDisplayText(text, totalWidth)
}

func formatUintCellNoHistory(value uint64, totalWidth int) string {
	return padDisplayText(fmt.Sprintf("[gray]%6d[none]", value), totalWidth)
}

func padDisplayText(text string, totalWidth int) string {
	if totalWidth <= 0 {
		return text
	}

	displayWidth := runewidth.StringWidth(stripColorTags(text))
	if displayWidth >= totalWidth {
		return text
	}
	return text + strings.Repeat(" ", totalWidth-displayWidth)
}

func stripColorTags(text string) string {
	return systemSummaryStyleTagPattern.ReplaceAllString(text, "")
}
