package list

import (
	"os"
	"strings"

	"github.com/blacknon/lssh/conf"
	termbox "github.com/nsf/termbox-go"
)

func pollEvent(serverNameList []string, cmdFlag bool, serverList conf.Config) (lineData []string) {
	defer termbox.Close()
	listData := getListData(serverNameList, serverList)
	selectline := 0
	headLine := 2

	_, height := termbox.Size()
	lineHeight := height - headLine

	searchText := ""
	allFlag := false

	filterListData := getFilterListData(searchText, listData)
	draw(filterListData, lineData, selectline, searchText)
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
				if selectline > 0 {
					selectline -= 1
				}
				draw(filterListData, lineData, selectline, searchText)

			// AllowDown Key
			case termbox.KeyArrowDown:
				if selectline < len(filterListData)-headLine {
					selectline += 1
				}
				draw(filterListData, lineData, selectline, searchText)

			// AllowRight Key
			case termbox.KeyArrowRight:
				if ((selectline+lineHeight)/lineHeight)*lineHeight <= len(filterListData) {
					selectline = ((selectline + lineHeight) / lineHeight) * lineHeight
				}
				draw(filterListData, lineData, selectline, searchText)

			// AllowLeft Key
			case termbox.KeyArrowLeft:
				if ((selectline-lineHeight)/lineHeight)*lineHeight >= 0 {
					selectline = ((selectline - lineHeight) / lineHeight) * lineHeight
				}

				draw(filterListData, lineData, selectline, searchText)

			// Ctrl + x Key(select)
			case termbox.KeyCtrlX:
				if cmdFlag == true {
					lineData = toggleList(lineData, strings.Fields(filterListData[selectline+1])[0])
				}

				draw(filterListData, lineData, selectline, searchText)

			// Ctrl + a Key(all select)
			case termbox.KeyCtrlA:
				if cmdFlag == true {
					lineData = allToggle(allFlag, lineData, filterListData[1:])
				}

				// allFlag Toggle
				if allFlag == false {
					allFlag = true
				} else {
					allFlag = false
				}

				draw(filterListData, lineData, selectline, searchText)

			// Enter Key
			case termbox.KeyEnter:
				if len(lineData) == 0 {
					lineData = append(lineData, strings.Fields(filterListData[selectline+1])[0])
				}
				return

			// BackSpace Key
			case termbox.KeyBackspace, termbox.KeyBackspace2:
				if len(searchText) > 0 {
					searchText = deleteRune(searchText)
					filterListData = getFilterListData(searchText, listData)
					if selectline > len(filterListData) {
						selectline = len(filterListData)
					}
					if selectline < 0 {
						selectline = 0
					}
					allFlag = false
					draw(filterListData, lineData, selectline, searchText)
				}

			// Space Key
			case termbox.KeySpace:
				searchText = searchText + " "
				draw(filterListData, lineData, selectline, searchText)

			// Other Key
			default:
				if ev.Ch != 0 {
					searchText = insertRune(searchText, ev.Ch)
					filterListData = getFilterListData(searchText, listData)
					if selectline > len(filterListData)-headLine {
						selectline = len(filterListData) - headLine
					}
					allFlag = false
					draw(filterListData, lineData, selectline, searchText)
				}
			}
		default:
			draw(filterListData, lineData, selectline, searchText)
		}
	}
}
