package monitor

import (
	"fmt"
	"sort"
	"strings"

	conf "github.com/blacknon/lssh/internal/config"
	mview "github.com/blacknon/mview"
	"github.com/gdamore/tcell/v2"
)

var cpuGraphPalette = []tcell.Color{
	tcell.NewRGBColor(0, 255, 255),
	tcell.NewRGBColor(0, 200, 120),
	tcell.NewRGBColor(255, 220, 0),
	tcell.NewRGBColor(255, 105, 180),
	tcell.NewRGBColor(255, 140, 0),
	tcell.NewRGBColor(80, 160, 255),
	tcell.NewRGBColor(180, 255, 120),
	tcell.NewRGBColor(255, 160, 220),
	tcell.NewRGBColor(0, 220, 200),
	tcell.NewRGBColor(255, 200, 80),
	tcell.NewRGBColor(140, 160, 255),
	tcell.NewRGBColor(255, 120, 120),
	tcell.NewRGBColor(180, 120, 255),
	tcell.NewRGBColor(120, 255, 180),
	tcell.NewRGBColor(255, 180, 120),
	tcell.NewRGBColor(120, 220, 255),
	tcell.NewRGBColor(255, 120, 200),
	tcell.NewRGBColor(160, 255, 80),
}

type cpuGraphTab struct {
	*mview.Box
	monitor *Monitor
	title   string
	history int
	panels  []metricGraphPanel
}

type cpuGraphSeriesLabel struct {
	row   int
	color tcell.Color
	text  string
}

type metricGraphPanel struct {
	Title     string
	Metric    string
	Height    int
	HistoryFn func(node *Node, limit int) ([]float64, error)
}

func newCPUGraphTab(m *Monitor) *cpuGraphTab {
	panels := buildMetricGraphPanels(m.r.Conf.Monitor.GraphTab)
	if len(panels) == 0 {
		return nil
	}

	box := mview.NewBox()
	box.SetBorder(true)
	box.SetBorderColor(monitorBorderColor)
	box.SetTitle("Graph")
	box.SetTitleAlign(mview.AlignLeft)
	box.SetTitleColor(monitorAccentColor)

	return &cpuGraphTab{
		Box:     box,
		monitor: m,
		title:   "Graph",
		history: 60,
		panels:  panels,
	}
}

func (g *cpuGraphTab) Draw(screen tcell.Screen) {
	if !g.GetVisible() {
		return
	}

	g.Box.Draw(screen)

	x, y, width, height := g.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}

	fillRect(screen, x, y, width, height, monitorBaseColor)

	nodes := g.snapshotNodes()
	selected := g.monitor.getSelectedNode()

	legendRows := g.drawLegend(screen, x, y, width, nodes, selected)
	graphY := y + legendRows
	graphHeight := height - legendRows
	if graphHeight < 5 || len(g.panels) == 0 {
		return
	}

	g.drawMetricSections(screen, x, graphY, width, graphHeight, nodes, selected)
}

func (g *cpuGraphTab) snapshotNodes() []*Node {
	nodes := append([]*Node(nil), g.monitor.Nodes...)
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i] == nil {
			return false
		}
		if nodes[j] == nil {
			return true
		}
		return nodes[i].ServerName < nodes[j].ServerName
	})
	return nodes
}

func (g *cpuGraphTab) drawLegend(screen tcell.Screen, x, y, width int, nodes []*Node, selected string) int {
	if width <= 0 {
		return 0
	}

	cursorX := x
	cursorY := y
	rows := 1

	for index, node := range nodes {
		if node == nil {
			continue
		}

		color := g.monitor.cpuGraphColorForServer(node.ServerName)
		label := node.ServerName
		if strings.TrimSpace(node.ServerName) == selected {
			label = "* " + label
			color = monitorAccentColor
		}
		segment := fmt.Sprintf("[%s] %s", cpuGraphLabel(index), label)
		if index < len(nodes)-1 {
			segment += "  "
		}

		segmentWidth := len(segment)
		if cursorX > x && cursorX+segmentWidth > x+width {
			cursorY++
			cursorX = x
			rows++
		}

		drawStyledText(screen, cursorX, cursorY, segment, color)
		cursorX += segmentWidth
	}

	return rows + 1
}

func (g *cpuGraphTab) drawMetricSection(
	screen tcell.Screen,
	x, y, width, height int,
	title string,
	nodes []*Node,
	selected string,
	historyFn func(node *Node, limit int) ([]float64, error),
) {
	if height < 5 {
		return
	}

	drawStyledText(screen, x, y, title, monitorAccentColor)
	g.drawGraph(screen, x, y+1, width, height-1, nodes, selected, historyFn)
}

func (g *cpuGraphTab) drawMetricSections(screen tcell.Screen, x, y, width, height int, nodes []*Node, selected string) {
	if len(g.panels) == 0 || height <= 0 {
		return
	}

	gapRows := max(0, len(g.panels)-1)
	availableHeight := height - gapRows
	if availableHeight <= 0 {
		return
	}

	totalWeight := 0
	for _, panel := range g.panels {
		totalWeight += panel.Height
	}
	if totalWeight <= 0 {
		totalWeight = len(g.panels)
	}

	currentY := y
	remainingHeight := availableHeight
	remainingWeight := totalWeight

	for index, panel := range g.panels {
		sectionHeight := remainingHeight
		if index < len(g.panels)-1 && remainingWeight > 0 {
			sectionHeight = max(5, remainingHeight*panel.Height/remainingWeight)
			maxAllowed := remainingHeight - (len(g.panels)-index-1)*5
			if sectionHeight > maxAllowed {
				sectionHeight = maxAllowed
			}
		}

		g.drawMetricSection(screen, x, currentY, width, sectionHeight, panel.Title, nodes, selected, panel.HistoryFn)

		currentY += sectionHeight + 1
		remainingHeight -= sectionHeight
		remainingWeight -= panel.Height
	}
}

func (g *cpuGraphTab) drawGraph(
	screen tcell.Screen,
	x, y, width, height int,
	nodes []*Node,
	selected string,
	historyFn func(node *Node, limit int) ([]float64, error),
) {
	const yAxisWidth = 5

	if width <= yAxisWidth+2 || height <= 2 {
		return
	}

	graphX := x + yAxisWidth
	graphWidth := width - yAxisWidth
	graphBottom := y + height - 1
	graphRows := height - 1
	virtualWidth := graphWidth * 2
	virtualHeight := graphRows * 4

	g.drawYAxis(screen, x, y, yAxisWidth-1, height)
	for col := 0; col < graphWidth; col++ {
		screen.SetContent(graphX+col, graphBottom, '─', nil, tcell.StyleDefault.Foreground(monitorBorderColor).Background(monitorBaseColor))
	}
	screen.SetContent(graphX-1, graphBottom, '└', nil, tcell.StyleDefault.Foreground(monitorBorderColor).Background(monitorBaseColor))

	canvas := newBrailleCanvas(graphWidth, graphRows)
	labels := make([]cpuGraphSeriesLabel, 0, len(nodes))

	for index, node := range nodes {
		if node == nil {
			continue
		}

		history, err := historyFn(node, g.history)
		if err != nil || len(history) == 0 {
			continue
		}

		color := g.monitor.cpuGraphColorForServer(node.ServerName)
		if strings.TrimSpace(node.ServerName) == selected {
			color = monitorAccentColor
		}

		samples := scaleCPUHistory(history, virtualWidth)
		canvas.drawSeries(samples, virtualHeight, color)
		labels = append(labels, cpuGraphSeriesLabel{
			row:   cpuGraphLabelRow(samples, virtualHeight, graphRows),
			color: color,
			text:  cpuGraphLabel(index),
		})
	}

	for row := 0; row < graphRows; row++ {
		for col := 0; col < graphWidth; col++ {
			cell := canvas.cells[row][col]
			if cell.mask == 0 {
				continue
			}

			screen.SetContent(
				graphX+col,
				y+row,
				rune(0x2800+cell.mask),
				nil,
				tcell.StyleDefault.Foreground(cell.color).Background(monitorBaseColor),
			)
		}
	}

	g.drawSeriesLabels(screen, graphX, y, graphWidth, graphRows, labels)
}

func (g *cpuGraphTab) drawYAxis(screen tcell.Screen, x, y, width, height int) {
	labels := []struct {
		value int
		row   int
	}{
		{100, y},
		{50, y + max(0, (height-2)/2)},
		{0, y + height - 2},
	}

	for _, label := range labels {
		text := fmt.Sprintf("%3d│", label.value)
		drawStyledText(screen, x, label.row, text, monitorMutedColor)
	}
}

func (g *cpuGraphTab) drawSeriesLabels(screen tcell.Screen, graphX, graphY, graphWidth, graphRows int, labels []cpuGraphSeriesLabel) {
	if graphWidth <= 0 || graphRows <= 0 || len(labels) == 0 {
		return
	}

	labelX := graphX + max(0, graphWidth-2)

	for _, label := range labels {
		row := label.row
		if row < 0 {
			row = 0
		}
		if row >= graphRows {
			row = graphRows - 1
		}
		drawStyledText(screen, labelX, graphY+row, label.text, label.color)
	}
}

func scaleCPUHistory(history []float64, width int) []float64 {
	if len(history) == 0 || width <= 0 {
		return []float64{}
	}

	if len(history) == 1 {
		result := make([]float64, width)
		for i := range result {
			result[i] = history[0]
		}
		return result
	}

	result := make([]float64, width)
	if len(history) <= width {
		lastIndex := len(history) - 1
		for i := 0; i < width; i++ {
			index := i * lastIndex / max(1, width-1)
			result[i] = history[index]
		}
		return result
	}

	for i := range result {
		start := i * len(history) / width
		end := (i + 1) * len(history) / width
		if end <= start {
			end = start + 1
		}
		sum := 0.0
		count := 0
		for _, value := range history[start:end] {
			sum += value
			count++
		}
		if count > 0 {
			result[i] = sum / float64(count)
		}
	}

	return result
}

func cpuGraphLabel(index int) string {
	if index < 26 {
		return string(rune('A' + index))
	}

	return fmt.Sprintf("%c%d", rune('A'+(index%26)), index/26)
}

func buildMetricGraphPanels(config conf.MonitorGraphTabConfig) []metricGraphPanel {
	panels := make([]metricGraphPanel, 0, len(config.Panels))
	for _, panelConfig := range config.Panels {
		panel, ok := newMetricGraphPanel(panelConfig)
		if !ok {
			continue
		}
		panels = append(panels, panel)
	}
	return panels
}

func newMetricGraphPanel(config conf.MonitorGraphTabPanelConfig) (metricGraphPanel, bool) {
	metric := strings.TrimSpace(strings.ToLower(config.Metric))
	title := strings.TrimSpace(config.Title)
	height := config.Height
	if height <= 0 {
		height = 1
	}

	switch metric {
	case "cpu":
		if title == "" {
			title = "CPU"
		}
		return metricGraphPanel{
			Title:  title,
			Metric: metric,
			Height: height,
			HistoryFn: func(node *Node, limit int) ([]float64, error) {
				return node.GetCPUUsageHistory(limit)
			},
		}, true
	case "mem", "memory":
		if title == "" {
			title = "Mem"
		}
		return metricGraphPanel{
			Title:  title,
			Metric: metric,
			Height: height,
			HistoryFn: func(node *Node, limit int) ([]float64, error) {
				return node.GetMemoryUsagePercentHistory(limit)
			},
		}, true
	default:
		return metricGraphPanel{}, false
	}
}

func cpuGraphLabelRow(samples []float64, virtualHeight, graphRows int) int {
	if len(samples) == 0 || graphRows <= 0 {
		return 0
	}

	virtualY := cpuValueToVirtualY(samples[len(samples)-1], virtualHeight)
	row := virtualY / 4
	if row < 0 {
		return 0
	}
	if row >= graphRows {
		return graphRows - 1
	}
	return row
}

func cpuValueToRow(value float64, height int) int {
	if height <= 1 {
		return 0
	}

	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}

	return int((value / 100.0) * float64(height-1))
}

func drawStyledText(screen tcell.Screen, x, y int, text string, color tcell.Color) {
	style := tcell.StyleDefault.Foreground(color).Background(monitorBaseColor)
	for offset, r := range text {
		screen.SetContent(x+offset, y, r, nil, style)
	}
}

func fillRect(screen tcell.Screen, x, y, width, height int, color tcell.Color) {
	style := tcell.StyleDefault.Background(color)
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			screen.SetContent(x+col, y+row, ' ', nil, style)
		}
	}
}

type brailleCanvas struct {
	width int
	rows  int
	cells [][]brailleCell
}

type brailleCell struct {
	mask  int
	color tcell.Color
}

func newBrailleCanvas(width, rows int) *brailleCanvas {
	cells := make([][]brailleCell, rows)
	for row := range cells {
		cells[row] = make([]brailleCell, width)
	}
	return &brailleCanvas{
		width: width,
		rows:  rows,
		cells: cells,
	}
}

func (c *brailleCanvas) drawSeries(samples []float64, virtualHeight int, color tcell.Color) {
	if len(samples) == 0 || c.width <= 0 || c.rows <= 0 || virtualHeight <= 0 {
		return
	}

	if len(samples) == 1 {
		y := cpuValueToVirtualY(samples[0], virtualHeight)
		c.setPoint(0, y, color)
		return
	}

	prevX := 0
	prevY := cpuValueToVirtualY(samples[0], virtualHeight)
	c.setPoint(prevX, prevY, color)

	for i := 1; i < len(samples); i++ {
		nextX := i
		nextY := cpuValueToVirtualY(samples[i], virtualHeight)
		c.drawLine(prevX, prevY, nextX, nextY, color)
		prevX = nextX
		prevY = nextY
	}
}

func (c *brailleCanvas) drawLine(x0, y0, x1, y1 int, color tcell.Color) {
	dx := absInt(x1 - x0)
	dy := absInt(y1 - y0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx - dy

	for {
		c.setPoint(x0, y0, color)
		if x0 == x1 && y0 == y1 {
			break
		}

		e2 := err * 2
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

func (c *brailleCanvas) setPoint(px, py int, color tcell.Color) {
	if px < 0 || py < 0 {
		return
	}

	cellX := px / 2
	cellY := py / 4
	if cellX < 0 || cellX >= c.width || cellY < 0 || cellY >= c.rows {
		return
	}

	mask := brailleBitMask(px%2, py%4)
	if mask == 0 {
		return
	}

	c.cells[cellY][cellX] = brailleCell{
		mask:  c.cells[cellY][cellX].mask | mask,
		color: color,
	}
}

func cpuValueToVirtualY(value float64, virtualHeight int) int {
	if virtualHeight <= 1 {
		return 0
	}

	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}

	scaled := int((value / 100.0) * float64(virtualHeight-1))
	return virtualHeight - 1 - scaled
}

func brailleBitMask(subX, subY int) int {
	leftBits := []int{0x01, 0x02, 0x04, 0x40}
	rightBits := []int{0x08, 0x10, 0x20, 0x80}

	if subY < 0 || subY > 3 {
		return 0
	}

	if subX == 0 {
		return leftBits[subY]
	}
	return rightBits[subY]
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
