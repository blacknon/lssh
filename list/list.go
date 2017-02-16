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

func drawLine(x, y int, str string, colorNum int, backColorNum int) {
	color := termbox.Attribute(colorNum + 1)
	backColor := termbox.Attribute(backColorNum + 1)
	// View Multi-Byte
	for _, char := range str {
		termbox.SetCell(x, y, char, color, backColor)
		x += runewidth.RuneWidth(char)
	}
}

func drawFilterLine(x, y int, str string, colorNum int, backColorNum int, keywordColorNum int, searchText string) {
	// SearchText Bounds Space
	searchWords := strings.Fields(searchText)

	for i := 0; i < len(searchWords); i += 1 {
		searchLowLine := strings.ToLower(str)
		searchKeyword := strings.ToLower(searchWords[i])
		searchKeywordLen := len(searchKeyword)
		searchKeywordCount := strings.Count(searchLowLine, searchKeyword)

		charLocation := 0
		for j := 0; j < searchKeywordCount; j += 1 {
			searchLineData := ""

			// Countermeasure "slice bounds out of range"
			if charLocation < len(str) {
				searchLineData = str[charLocation:]
			}
			searchLineDataStr := string(searchLineData)
			searchKeywordIndex := strings.Index(strings.ToLower(searchLineDataStr), searchKeyword)

			charLocation = charLocation + searchKeywordIndex
			keyword := ""

			// Countermeasure "slice bounds out of range"
			if charLocation < len(str) {
				keyword = str[charLocation : charLocation+searchKeywordLen]
			}

			// Get Multibyte Charctor Location
			multibyteStrCheckLine := str[:charLocation]
			multiByteCharLocation := 0
			for _, multiByteChar := range multibyteStrCheckLine {
				multiByteCharLocation += runewidth.RuneWidth(multiByteChar)
			}

			drawLine(x+multiByteCharLocation, y, keyword, keywordColorNum, backColorNum)
			charLocation = charLocation + searchKeywordLen
		}
	}
}

// Draw List
func draw(serverNameList []string, selectCursor int, searchText string) {
	headLine := 2
	leftMargin := 2
	defaultColor := 255
	defaultBackColor := 255
	pronpt := "lssh>>"
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
	drawLine(0, 0, pronpt, 3, defaultBackColor)
	drawLine(len(pronpt), 0, searchText, defaultColor, defaultBackColor)
	drawLine(leftMargin, 1, serverNameList[0], 3, defaultBackColor)

	// View List
	for listKey, listValue := range serverViewList {
		// Set cursor color
		cursorColor := defaultColor
		cursorBackColor := defaultBackColor
		keywordColor := 1
		if listKey == selectViewCursor {
			// Select line color
			cursorColor = 0
			cursorBackColor = 2
		}

		// Draw filter line
		drawLine(leftMargin, listKey+headLine, listValue, cursorColor, cursorBackColor)
		drawFilterLine(leftMargin, listKey+headLine, listValue, cursorColor, cursorBackColor, keywordColor, searchText)
		listKey += 1
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
	tabWriterBuffer := new(tabwriter.Writer)
	tabWriterBuffer.Init(buffer, 0, 4, 8, ' ', 0)
	fmt.Fprintln(tabWriterBuffer, "ServerName \tConnect Infomation \tNote \t")

	for _, key := range serverNameList {
		serverName := key
		connectInfomation := serverList.Server[key].User + "@" + serverList.Server[key].Addr
		serverNote := serverList.Server[key].Note
		fmt.Fprintln(tabWriterBuffer, serverName+"\t"+connectInfomation+"\t"+serverNote+"\t")
	}
	tabWriterBuffer.Flush()
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

func getFilterListData(searchText string, listData []string) (returnListData []string) {
	// SearchText Bounds Space
	searchWords := strings.Fields(searchText)
	r := listData[1:]
	line := ""
	loopListData := []string{}
	returnListData = append(returnListData, listData[0])

	// if No searchWords
	if len(searchWords) == 0 {
		returnListData = listData
		return returnListData
	}

	for i := 0; i < len(searchWords); i += 1 {
		searchWordMeta := regexp.QuoteMeta(strings.ToLower(searchWords[i]))
		re := regexp.MustCompile(searchWordMeta)
		loopListData = []string{}

		for j := 0; j < len(r); j += 1 {
			line += string(r[j])
			if re.MatchString(strings.ToLower(line)) {
				loopListData = append(loopListData, line)
			}
			line = ""
		}
		r = loopListData
	}
	returnListData = append(returnListData, loopListData...)
	return returnListData
}

func pollEvent(serverNameList []string, serverList conf.Config) (lineData string) {
	defer termbox.Close()
	listData := getListData(serverNameList, serverList)
	selectline := 0
	headLine := 2

	_, height := termbox.Size()
	lineHeight := height - headLine

	searchText := ""

	filterListData := getFilterListData(searchText, listData)
	draw(filterListData, selectline, searchText)
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
				draw(filterListData, selectline, searchText)

			// AllowDown Key
			case termbox.KeyArrowDown:
				if selectline < len(filterListData)-headLine {
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

			// Space Key
			case termbox.KeySpace:
				searchText = searchText + " "
				draw(filterListData, selectline, searchText)

			// Other Key
			default:
				if ev.Ch != 0 {
					searchText = insertRune(searchText, ev.Ch)
					filterListData = getFilterListData(searchText, listData)
					if selectline > len(filterListData)-headLine {
						selectline = len(filterListData) - headLine
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
