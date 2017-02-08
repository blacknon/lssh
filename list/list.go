package list

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/blacknon/lssh/conf"
	runewidth "github.com/mattn/go-runewidth"
	termbox "github.com/nsf/termbox-go"
)

type ListArrayInfo struct {
	Name    string
	Connect string
	Note    string
}

// Draw Line
func drawLine(x, y int, str string, colorNum int, backColorNum int) {
	color := termbox.Attribute(colorNum + 1)
	backColor := termbox.Attribute(backColorNum + 1)
	// View Multi-Byte
	for _, c := range str {
		termbox.SetCell(x, y, c, color, backColor)
		x += runewidth.RuneWidth(c)
	}
}

// Draw List
func draw(serverNameList []string, selectCursor int, searchText string) {
	headLine := 2
	defaultColor := 255
	defaultBackColor := 255
	termbox.Clear(termbox.Attribute(defaultColor+1), termbox.Attribute(defaultBackColor+1))

	// Get Terminal Size
	_, height := termbox.Size()
	lineHeight := height - headLine

	// Set View List Range
	viewFirstLine := (selectCursor/lineHeight)*lineHeight + 1
	viewLastLine := viewFirstLine + lineHeight
	var serverViewList []string
	if viewLastLine > len(serverNameList) {
		serverViewList = serverNameList[viewFirstLine:]
	} else {
		serverViewList = serverNameList[viewFirstLine:viewLastLine]
	}
	selectViewCursor := selectCursor - viewFirstLine + 1

	// View Head
	pronpt := "lssh>>"
	drawLine(0, 0, pronpt, 3, defaultBackColor)
	drawLine(len(pronpt), 0, searchText, defaultColor, defaultBackColor)
	drawLine(headLine, 1, serverNameList[0], defaultColor, defaultBackColor)

	// View List
	for k, v := range serverViewList {
		cursorColor := defaultColor
		cursorBackColor := defaultBackColor
		if k == selectViewCursor {
			cursorColor = 0
			cursorBackColor = 2
		}

		viewListData := v
		drawLine(2, k+2, viewListData, cursorColor, cursorBackColor)
		k += 1
	}

	// Multi-Byte SetCursor
	x := 0
	for _, c := range searchText {
		x += runewidth.RuneWidth(c)
	}
	termbox.SetCursor(len(pronpt)+x, 0)
	termbox.Flush()
}

// Create View List Data (use text/tabwriter)
func getListData(serverNameList []string, serverList conf.Config) (listData []string) {
	buffer := &bytes.Buffer{}
	w := new(tabwriter.Writer)
	w.Init(buffer, 0, 4, 8, ' ', 0)
	fmt.Fprintln(w, "ServerName \tConnect Infomation \tNote \t")

	for _, v := range serverNameList {
		fmt.Fprintln(w, v+"\t"+serverList.Server[v].User+"@"+serverList.Server[v].Addr+"\t"+serverList.Server[v].Note+"\t")
	}
	w.Flush()
	line, err := buffer.ReadString('\n')
	for err == nil {
		str := strings.Replace(line, "\t", " ", -1)
		listData = append(listData, str)
		line, err = buffer.ReadString('\n')
	}
	return listData
}

func insertRune(text string, inputRune rune) (returnText string) {
	returnText = text + string(inputRune)
	return
}

func deleteRune(text string) (returnText string) {
	s := text
	sc := []rune(s)
	returnText = string(sc[:(len(sc) - 1)])
	return
}

func getFilterListData(searchText string, listData []string) (retrunListData []string) {
	searchTextMeta := regexp.QuoteMeta(strings.ToLower(searchText))
	re := regexp.MustCompile(searchTextMeta)
	r := listData[1:]
	line := ""

	retrunListData = append(retrunListData, listData[0])
	for i := 0; i < len(r); i += 1 {
		line += string(r[i])
		if re.MatchString(strings.ToLower(line)) {
			retrunListData = append(retrunListData, line)
		}
		line = ""
	}
	return retrunListData
}

func pollEvent(serverNameList []string, serverList conf.Config) (lineData string) {
	defer termbox.Close()
	listData := getListData(serverNameList, serverList)
	selectline := 0

	_, height := termbox.Size()
	lineHeight := height - 2

	searchText := ""
	filterListData := getFilterListData(searchText, listData)
	draw(filterListData, selectline, searchText)
	for {
		switch ev := termbox.PollEvent(); ev.Type {
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
				draw(filterListData, selectline, searchText)

			// AllowDown Key
			case termbox.KeyArrowDown:
				if selectline < len(filterListData)-2 {
					selectline += 1
				}
				draw(filterListData, selectline, searchText)

			// AllowRight Key
			case termbox.KeyArrowRight:
				if ((selectline+lineHeight)/lineHeight)*lineHeight <= len(filterListData) {
					selectline = ((selectline + lineHeight) / lineHeight) * lineHeight
				}
				draw(filterListData, selectline, searchText)

			// AllowLeft Key
			case termbox.KeyArrowLeft:
				if ((selectline-lineHeight)/lineHeight)*lineHeight >= 0 {
					selectline = ((selectline - lineHeight) / lineHeight) * lineHeight
				}

				draw(filterListData, selectline, searchText)

			// Enter Key
			case termbox.KeyEnter:
				lineData = strings.Fields(filterListData[selectline+1])[0]
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
					draw(filterListData, selectline, searchText)
				}

			// Other Key
			default:
				if ev.Ch != 0 {
					searchText = insertRune(searchText, ev.Ch)
					filterListData = getFilterListData(searchText, listData)
					if selectline > len(filterListData)-2 {
						selectline = len(filterListData) - 2
					}
					draw(filterListData, selectline, searchText)
				}
			}
		default:
			draw(filterListData, selectline, searchText)
		}
	}
}

func DrawList(serverNameList []string, serverList conf.Config) (lineName string) {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}

	lineName = pollEvent(serverNameList, serverList)
	return lineName
}
