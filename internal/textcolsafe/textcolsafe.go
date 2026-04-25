// Safe column printing to avoid calling external `stty` on Windows.
// Reference: https://github.com/blacknon/lssh/issues/146
package textcolsafe

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/acarl005/stripansi"
)

// PrintColumns prints items into columns safely without relying on external `stty`.
// - w: output writer
// - strs: items to print
// - margin: spaces between columns
// - padding: left padding to subtract from terminal width (e.g., prompt length)
func PrintColumns(w io.Writer, strs []string, margin int, padding int) {
	maxLength := 0
	marginStr := strings.Repeat(" ", margin)
	lengths := []int{}
	for _, str := range strs {
		colorless := stripansi.Strip(str)
		length := utf8.RuneCountInString(colorless)
		if length > maxLength {
			maxLength = length
		}
		lengths = append(lengths, length)
	}

	width := getTermWidth()
	width = width - padding

	numCols, numRows := calculateTableSize(width, margin, maxLength, len(strs))

	if numCols == 1 {
		for _, str := range strs {
			fmt.Fprintln(w, str)
		}
		return
	}

	for i := 0; i < numCols*numRows; i++ {
		x, y := rowIndexToTableCoords(i, numCols)
		j := tableCoordsToColIndex(x, y, numRows)

		strLen := 0
		str := ""
		if j < len(lengths) {
			strLen = lengths[j]
			str = strs[j]
		}

		numSpacesRequired := maxLength - strLen
		spaceStr := strings.Repeat(" ", numSpacesRequired)

		_, _ = io.WriteString(w, str)

		if x+1 == numCols {
			_, _ = io.WriteString(w, "\n")
		} else {
			_, _ = io.WriteString(w, spaceStr)
			_, _ = io.WriteString(w, marginStr)
		}
	}
}

func getTermWidth() int {
	if col := os.Getenv("COLUMNS"); col != "" {
		if w, err := strconv.Atoi(col); err == nil {
			return w
		}
	}
	return 80
}

func calculateTableSize(width, margin, maxLength, numCells int) (int, int) {
	numCols := (width + margin) / (maxLength + margin)
	if numCols == 0 {
		numCols = 1
	}
	numRows := (numCells + numCols - 1) / numCols
	return numCols, numRows
}

func rowIndexToTableCoords(i, numCols int) (int, int) {
	x := i % numCols
	y := i / numCols
	return x, y
}

func tableCoordsToColIndex(x, y, numRows int) int {
	return y + numRows*x
}
