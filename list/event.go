package list

import (
	"os"
	"strings"

	termbox "github.com/nsf/termbox-go"
)

func (l *ListInfo) insertRune(inputRune rune) {
	l.Keyword = l.Keyword + string(inputRune)

}

func (l *ListInfo) deleteRune() {
	sc := []rune(l.Keyword)
	l.Keyword = string(sc[:(len(sc) - 1)])
}

//func (l *ListInfo) keyEvent() (lineData []string) {
func (l *ListInfo) keyEvent() (lineData []string) {
	l.CursorLine = 0
	headLine := 2

	_, height := termbox.Size()
	height = height - headLine

	l.Keyword = ""
	allFlag := false // input Ctrl + A flag

	l.getFilterText()
	l.draw()

	for {
		switch ev := termbox.PollEvent(); ev.Type {

		// Get Key Event
		case termbox.EventKey:
			switch ev.Key {
			// ESC or Ctrl + C Key (Exit)
			case termbox.KeyEsc, termbox.KeyCtrlC:
				termbox.Close()
				os.Exit(0)

			// AllowUp Key
			case termbox.KeyArrowUp:
				if l.CursorLine > 0 {
					l.CursorLine -= 1
				}
				l.draw()

			// AllowDown Key
			case termbox.KeyArrowDown:
				if l.CursorLine < len(l.ViewText)-headLine {
					l.CursorLine += 1
				}
				l.draw()

			// AllowRight Key
			case termbox.KeyArrowRight:
				nextPosition := ((l.CursorLine + height) / height) * height
				if nextPosition+2 <= len(l.ViewText) {
					l.CursorLine = nextPosition
				}
				l.draw()

			// AllowLeft Key
			case termbox.KeyArrowLeft:
				beforePosition := ((l.CursorLine - height) / height) * height
				if beforePosition >= 0 {
					l.CursorLine = beforePosition
				}
				l.draw()

			// Tab Key(select)
			case termbox.KeyTab:
				if l.MultiFlag == true {
					l.toggle(strings.Fields(l.ViewText[l.CursorLine+1])[0])
				}
				if l.CursorLine < len(l.ViewText)-headLine {
					l.CursorLine += 1
				}
				l.draw()

			// Ctrl + a Key(all select)
			case termbox.KeyCtrlA:
				if l.MultiFlag == true {
					l.allToggle(allFlag)
					// allFlag Toggle
					if allFlag == false {
						allFlag = true
					} else {
						allFlag = false
					}
				}
				l.draw()

			// Ctrl + h Key(Help Window)
			//case termbox.KeyCtrlH:

			// Enter Key
			case termbox.KeyEnter:
				if len(l.SelectName) == 0 {
					l.SelectName = append(l.SelectName, strings.Fields(l.ViewText[l.CursorLine+1])[0])
				}
				return

			// BackSpace Key
			case termbox.KeyBackspace, termbox.KeyBackspace2:
				if len(l.Keyword) > 0 {
					l.deleteRune()

					l.getFilterText()
					if l.CursorLine > len(l.ViewText) {
						l.CursorLine = len(l.ViewText)
					}
					if l.CursorLine < 0 {
						l.CursorLine = 0
					}
					allFlag = false
					l.draw()
				}

			// Space Key
			case termbox.KeySpace:
				l.Keyword = l.Keyword + " "
				l.draw()

			// Other Key
			default:
				if ev.Ch != 0 {
					l.insertRune(ev.Ch)
					l.getFilterText()
					if l.CursorLine > len(l.ViewText)-headLine {
						l.CursorLine = len(l.ViewText) - headLine
					}
					allFlag = false
					l.draw()
				}
			}
		default:
			l.draw()
		}
	}
}
