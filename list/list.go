package list

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/blacknon/lssh/conf"
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
	runes := []rune(str)

	for i := 0; i < len(runes); i += 1 {
		termbox.SetCell(x+i, y, runes[i], color, backColor)
	}
}

// Draw List
func draw(serverNameList []string, selectCursor int, searchText string) {
	defaultColor := 255
	defaultBackColor := 255
	termbox.Clear(termbox.Attribute(defaultColor+1), termbox.Attribute(defaultBackColor+1))

	// Get Terminal Size
	_, height := termbox.Size()
	lineHeight := height - 2

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
	drawLine(0, 0, "lssh>"+searchText, defaultColor, defaultBackColor)
	drawLine(2, 1, serverNameList[0], defaultColor, defaultBackColor)

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

	termbox.SetCursor(5+len([]rune(searchText)), 0)
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
	re := regexp.MustCompile(searchText)
	r := listData[1:]
	line := ""

	retrunListData = append(retrunListData, listData[0])
	for i := 0; i < len(r); i += 1 {
		line += string(r[i])
		if re.MatchString(line) {
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
			case termbox.KeyEsc, termbox.KeyCtrlC:
				termbox.Close()
				os.Exit(0)
			case termbox.KeyArrowUp:
				if selectline > 0 {
					selectline -= 1
				}
				draw(filterListData, selectline, searchText)
			case termbox.KeyArrowDown:
				if selectline < len(filterListData)-1 {
					selectline += 1
				}
				draw(filterListData, selectline, searchText)
			case termbox.KeyArrowRight:
				if ((selectline+lineHeight)/lineHeight)*lineHeight <= len(filterListData) {
					selectline = ((selectline + lineHeight) / lineHeight) * lineHeight
				}
				draw(filterListData, selectline, searchText)
			case termbox.KeyArrowLeft:
				if ((selectline-lineHeight)/lineHeight)*lineHeight >= 0 {
					selectline = ((selectline - lineHeight) / lineHeight) * lineHeight
				}

				draw(filterListData, selectline, searchText)
			case termbox.KeyEnter:
				lineData = strings.Fields(filterListData[selectline+1])[0]
				return
			case termbox.KeyBackspace, termbox.KeyBackspace2:
				if len(searchText) > 0 {
					searchText = deleteRune(searchText)
					filterListData = getFilterListData(searchText, listData)
					if selectline > len(filterListData) {
						selectline = len(filterListData)
					}
					draw(filterListData, selectline, searchText)
				}
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
