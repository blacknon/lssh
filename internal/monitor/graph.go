// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"math"
)

const (
	ColorModeNone = iota
	ColorModePercentage
)

type Graph struct {
	// Data is the data to be graphed.
	Data []float64

	// Min and Max are the minimum and maximum values of the graph.
	Min float64
	Max float64

	// ColorMode
	ColorMode int
}

var (
	sparklineTicks = []string{
		"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█",
	}

	graphSymbols = map[string]string{
		"00": "⠀", "01": "⢀", "02": "⢠", "03": "⢰", "04": "⢸",
		"10": "⡀", "11": "⣀", "12": "⣠", "13": "⣰", "14": "⣸",
		"20": "⡄", "21": "⣄", "22": "⣤", "23": "⣴", "24": "⣼",
		"30": "⡆", "31": "⣆", "32": "⣦", "33": "⣶", "34": "⣾",
		"40": "⡇", "41": "⣇", "42": "⣧", "43": "⣷", "44": "⣿",
	}
)

func (g *Graph) Sparkline() (result []string) {

	if len(g.Data) == 0 {
		return
	}

	min := g.Min
	max := g.Max
	if min == 0 && max == 0 {
		min, max = g.Data[0], g.Data[0]
		for _, num := range g.Data {
			if num < min {
				min = num
			}
			if num > max {
				max = num
			}
		}
	}

	sizeRatio := float64(len(sparklineTicks)) / float64(len(g.Data))
	if sizeRatio > 1 {
		sizeRatio = 1
	}

	numTicks := int(sizeRatio * float64(len(sparklineTicks)))

	if numTicks < 1 {
		numTicks = 1
	}

	if math.Abs(max-min) < 0.0000001 {
		for range g.Data {
			result = append(result, sparklineTicks[0])
		}
	} else {
		scale := float64(numTicks-1) / (max - min)
		for _, n := range g.Data {
			tick := int((n-min)*scale + 0.5)
			if tick >= numTicks {
				tick = numTicks - 1
			}
			result = append(result, sparklineTicks[tick])
		}
	}

	return
}

func usageToSymbol(current, next float64) (symbol string, template string) {
	template = "%s"
	getSymbolIndex := func(value float64) int {
		if value < 25 {
			return 1
		} else if value < 50 {
			return 2
		} else if value < 75 {
			return 3
		} else {
			return 4
		}
	}

	currentIndex := getSymbolIndex(current)
	nextIndex := getSymbolIndex(next)

	switch {
	case currentIndex == 4 || nextIndex == 4:
		template = "[#fa1e1e]%s[none]"
	case currentIndex == 3 || nextIndex == 3:
		template = "[#E78101]%s[none]"
	case currentIndex == 2 || nextIndex == 2:
		template = "[#f2e266]%s[none]"
	case currentIndex == 1 || nextIndex == 1:
		template = "[#4897d4]%s[none]"
	case currentIndex == 0 && nextIndex == 0:
		template = "%s"
	default:
		template = "%s"
	}

	key := fmt.Sprintf("%d%d", currentIndex, nextIndex)
	return graphSymbols[key], template
}

func (g *Graph) BrailleLine() (result []string) {
	width := len(g.Data)
	if width == 0 {
		return
	}

	graphArray := make([]string, width)
	for i := 0; i < width; i += 2 {
		current := g.Data[i]
		next := 0.0
		if i < width-1 {
			next = g.Data[i+1]
		}

		symbol, template := usageToSymbol(current, next)
		graphArray[i] = fmt.Sprintf(template, symbol)
	}

	for _, symbol := range graphArray {
		result = append(result, symbol)
	}

	return
}
