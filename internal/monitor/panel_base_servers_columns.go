// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"strconv"

	mview "github.com/blacknon/mview"
	"github.com/dustin/go-humanize"
	"github.com/gdamore/tcell/v2"
)

func (m *Monitor) getBaseGridTableDataIsConnect(isConnect bool) (connectCell *mview.TableCell) {
	if isConnect {
		connectCell = mview.NewTableCell("[green]OK[none]")
		connectCell.Align = mview.AlignCenter
	} else {
		connectCell = mview.NewTableCell("[red]NG[none]")
		connectCell.Align = mview.AlignCenter
	}

	return
}

func (m *Monitor) getBaseGridTableDataUptime(isConnect bool, node *Node) (uptimeCell *mview.TableCell) {
	if isConnect {
		uptime, err := node.GetUptime()
		if err != nil || uptime == nil {
			uptimeCell = mview.NewTableCell("-")
			uptimeCell.Align = mview.AlignCenter
		} else {
			uptimeTotal := uptime.GetTotalDuration()
			uptimeCell = mview.NewTableCell(uptimeFormatDuration(uptimeTotal))
			uptimeCell.Align = mview.AlignRight
		}
	} else {
		uptimeCell = mview.NewTableCell("-")
		uptimeCell.Align = mview.AlignCenter
	}

	return
}

func (m *Monitor) getBaseGridTableDataCPUCore(isConnect bool, node *Node) (cpuCoreCell *mview.TableCell) {
	if isConnect {
		cpuCore, err := node.GetCPUCore()
		if err != nil {
			cpuCoreCell = mview.NewTableCell("-")
			cpuCoreCell.Align = mview.AlignCenter
		} else {
			cpuCoreCell = mview.NewTableCell(fmt.Sprintf("[gray]%6s[none]", strconv.Itoa(cpuCore)))
			cpuCoreCell.Align = mview.AlignRight
		}
	} else {
		cpuCoreCell = mview.NewTableCell("-")
		cpuCoreCell.Align = mview.AlignCenter
	}

	cpuCoreCell.SetTextColor(tcell.NewRGBColor(0, 255, 255))

	return
}

func (m *Monitor) getBaseGridTableDataCPUUsage(isConnect bool, node *Node) (cpuUsageCell *mview.TableCell) {
	if isConnect {
		cpuUsage, err := node.GetCPUUsage()

		if err != nil {
			cpuUsageCell = mview.NewTableCell("-")
			cpuUsageCell.Align = mview.AlignCenter
		} else {
			cpuUsageCell = mview.NewTableCell(fmt.Sprintf("[yellow]%8.2f[gray]%%[none]", cpuUsage))
			cpuUsageCell.Align = mview.AlignRight
		}
	} else {
		cpuUsageCell = mview.NewTableCell("-")
		cpuUsageCell.Align = mview.AlignCenter
	}

	return
}

func (m *Monitor) getBaseGridTableDataCPUUsageWithSparkline(isConnect bool, node *Node) (cpuUsageCell *mview.TableCell) {
	if isConnect {
		cpuUsage, sparkline, err := node.GetCPUUsageWithSparkline()

		if err != nil {
			cpuUsageCell = mview.NewTableCell("-")
			cpuUsageCell.Align = mview.AlignCenter
		} else {
			cpuUsageCell = mview.NewTableCell(fmt.Sprintf("[yellow]%8.2f[garay]%%[none] [gray]%s[none]", cpuUsage, sparkline))
			cpuUsageCell.Align = mview.AlignRight
		}
	} else {
		cpuUsageCell = mview.NewTableCell("-")
		cpuUsageCell.Align = mview.AlignCenter
	}

	return
}

func (m *Monitor) getBaseGridTableDataCPUUsageWithBrailleLine(isConnect bool, node *Node) (cpuUsageCell *mview.TableCell) {
	if isConnect {
		cpuUsage, sparkline, err := node.GetCPUUsageWithBrailleLine()

		if err != nil {
			cpuUsageCell = mview.NewTableCell("-")
			cpuUsageCell.Align = mview.AlignCenter
		} else {
			cpuUsageCell = mview.NewTableCell(fmt.Sprintf("[yellow]%8.2f[gray]%%[none] [gray]%s[none]", cpuUsage, sparkline))
			cpuUsageCell.Align = mview.AlignRight
		}
	} else {
		cpuUsageCell = mview.NewTableCell("-")
		cpuUsageCell.Align = mview.AlignCenter
	}

	return
}

func (m *Monitor) getBaseGridTableDataMemUsage(isConnect bool, node *Node) (memUseCell, memTotalCell, swapUseCell, swapTotalCell *mview.TableCell) {
	if isConnect {
		memUsed, memTotal, swapUsed, swapTotal, err := node.GetMemoryUsage()

		if err != nil {
			memUseCell = mview.NewTableCell("-")
			memUseCell.Align = mview.AlignCenter

			memTotalCell = mview.NewTableCell("-")
			memTotalCell.Align = mview.AlignCenter

			swapUseCell = mview.NewTableCell("-")
			swapUseCell.Align = mview.AlignCenter

			swapTotalCell = mview.NewTableCell("-")
			swapTotalCell.Align = mview.AlignCenter
		} else {
			humanizeMemUsed := humanize.Bytes(memUsed)
			humanizeMemTotal := humanize.Bytes(memTotal)
			humanizeSwapUsed := humanize.Bytes(swapUsed)
			humanizeSwapTotal := humanize.Bytes(swapTotal)

			memUseCell = mview.NewTableCell(fmt.Sprintf("%14s", humanizeMemUsed))
			memUseCell.Align = mview.AlignRight

			memTotalCell = mview.NewTableCell(fmt.Sprintf("%14s", humanizeMemTotal))
			memTotalCell.Align = mview.AlignRight

			swapUseCell = mview.NewTableCell(fmt.Sprintf("%14s", humanizeSwapUsed))
			swapUseCell.Align = mview.AlignRight

			swapTotalCell = mview.NewTableCell(fmt.Sprintf("%14s", humanizeSwapTotal))
			swapTotalCell.Align = mview.AlignRight
		}
	} else {
		memUseCell = mview.NewTableCell("-")
		memUseCell.Align = mview.AlignCenter

		memTotalCell = mview.NewTableCell("-")
		memTotalCell.Align = mview.AlignCenter

		swapUseCell = mview.NewTableCell("-")
		swapUseCell.Align = mview.AlignCenter

		swapTotalCell = mview.NewTableCell("-")
		swapTotalCell.Align = mview.AlignCenter
	}

	return
}

func (m *Monitor) getBaseGridTableDataTasks(isConnect bool, node *Node) (tasksCell *mview.TableCell) {
	if isConnect {
		tasks, err := node.GetTaskCounts()

		if err != nil {
			tasksCell = mview.NewTableCell("-")
			tasksCell.Align = mview.AlignCenter
		} else {
			tasksCell = mview.NewTableCell(fmt.Sprintf("%6d", tasks))
			tasksCell.Align = mview.AlignRight
		}
	} else {
		tasksCell = mview.NewTableCell("-")
		tasksCell.Align = mview.AlignCenter
	}

	return
}

func (m *Monitor) getBaseGridTableDataLoadAvg(isConnect bool, node *Node) (loadAvg15minCell, loadAvg5minCell, loadAvg1minCell *mview.TableCell) {
	if isConnect {
		loadAvg, err := node.GetLoadAvg()

		if err != nil || loadAvg == nil {
			loadAvg15minCell = mview.NewTableCell("-")
			loadAvg15minCell.Align = mview.AlignCenter

			loadAvg5minCell = mview.NewTableCell("-")
			loadAvg5minCell.Align = mview.AlignCenter

			loadAvg1minCell = mview.NewTableCell("-")
			loadAvg1minCell.Align = mview.AlignCenter

		} else {
			loadAvg15minCell = mview.NewTableCell(fmt.Sprintf("%14.2f", loadAvg.Last15Min))
			loadAvg15minCell.Align = mview.AlignRight

			loadAvg5minCell = mview.NewTableCell(fmt.Sprintf("%14.2f", loadAvg.Last5Min))
			loadAvg5minCell.Align = mview.AlignRight

			loadAvg1minCell = mview.NewTableCell(fmt.Sprintf("%14.2f", loadAvg.Last1Min))
			loadAvg1minCell.Align = mview.AlignRight
		}
	} else {
		loadAvg15minCell = mview.NewTableCell("-")
		loadAvg15minCell.Align = mview.AlignCenter

		loadAvg5minCell = mview.NewTableCell("-")
		loadAvg5minCell.Align = mview.AlignCenter

		loadAvg1minCell = mview.NewTableCell("-")
		loadAvg1minCell.Align = mview.AlignCenter
	}

	return
}
