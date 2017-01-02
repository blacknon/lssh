package list

import (
	"bytes"
	"fmt"
	"os"
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

func drawLine(x, y int, str string, colorNum int, backColorNum int) {
	color := termbox.Attribute(colorNum + 1)
	backColor := termbox.Attribute(backColorNum + 1)
	runes := []rune(str)

	for i := 0; i < len(runes); i += 1 {
		termbox.SetCell(x+i, y, runes[i], color, backColor)
	}
}

func draw(serverNameList []string, selectCursor int) {
	defaultColor := 255
	defaultBackColor := 255
	termbox.Clear(termbox.Attribute(defaultColor+1), termbox.Attribute(defaultBackColor+1))

	_, height := termbox.Size()
	lineHeight := height - 2

	viewFirstLine := (selectCursor/lineHeight)*lineHeight + 1
	viewLastLine := viewFirstLine + lineHeight
	drawLine(0, 0, "lssh>", defaultColor, defaultBackColor)
	drawLine(2, 1, serverNameList[0], defaultColor, defaultBackColor)

	serverViewList := serverNameList[viewFirstLine:viewLastLine]
	selectViewCursor := selectCursor - viewFirstLine + 1

	for k, v := range serverViewList {
		cursorColor := defaultColor
		cursorBackColor := defaultBackColor
		if k == selectViewCursor {
			cursorColor = 0
			cursorBackColor = 2
		}

		serverName := v

		drawLine(2, k+2, serverName, cursorColor, cursorBackColor)
		k += 1
	}

	termbox.Flush()
}

func GetListData(serverNameList []string, serverList conf.Config) (listData []string) {
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

func pollEvent(serverNameList []string, serverList conf.Config) (selectline int) {
	defer termbox.Close()
	listData := GetListData(serverNameList, serverList)
	selectline = 0

	_, height := termbox.Size()
	lineHeight := height - 2

	termbox.SetCursor(5, 0)
	draw(listData, selectline)
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
				draw(listData, selectline)
			case termbox.KeyArrowDown:
				if selectline < len(serverNameList)-1 {
					selectline += 1
				}
				draw(listData, selectline)
			case termbox.KeyArrowRight:
				if ((selectline+lineHeight)/lineHeight)*lineHeight <= len(serverNameList) {
					selectline = ((selectline + lineHeight) / lineHeight) * lineHeight
				}
				draw(listData, selectline)
			case termbox.KeyArrowLeft:
				if ((selectline-lineHeight)/lineHeight)*lineHeight >= 0 {
					selectline = ((selectline - lineHeight) / lineHeight) * lineHeight
				}

				draw(listData, selectline)
			case termbox.KeyEnter:
				return selectline
			default:
				draw(listData, selectline)
			}
		default:
			draw(listData, selectline)
		}
	}
}

func DrawList(serverNameList []string, serverList conf.Config) (lineNo int) {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}

	lineNo = pollEvent(serverNameList, serverList)

	return
}
