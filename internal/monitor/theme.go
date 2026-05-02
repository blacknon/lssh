package monitor

import (
	mview "github.com/blacknon/mview"
	"github.com/gdamore/tcell/v2"
)

var (
	monitorAccentColor = tcell.NewRGBColor(0, 255, 255)
	monitorHeaderColor = tcell.ColorGreen
	monitorBorderColor = tcell.ColorDarkGray
	monitorTextColor   = tcell.ColorWhite
	monitorMutedColor  = tcell.ColorGray
	monitorBaseColor   = tcell.ColorBlack
)

func applyMonitorTableStyle(table *mview.Table, selectable bool) {
	table.SetBorder(false)
	table.SetBackgroundColor(mview.ColorUnset)
	table.SetSelectedStyle(monitorBaseColor, monitorAccentColor, tcell.AttrNone)
	table.SetSelectable(selectable, false)
}

func newMonitorHeaderCell(text string) *mview.TableCell {
	cell := mview.NewTableCell(text)
	cell.SetTextColor(monitorBaseColor)
	cell.SetBackgroundColor(monitorHeaderColor)
	cell.SetAlign(mview.AlignLeft)
	cell.SetSelectable(false)
	cell.SetIsHeader(true)
	return cell
}

func setMonitorAccentText(cell *mview.TableCell) {
	if cell != nil {
		cell.SetTextColor(monitorAccentColor)
	}
}

func setMonitorMutedText(cell *mview.TableCell) {
	if cell != nil {
		cell.SetTextColor(monitorMutedColor)
	}
}
