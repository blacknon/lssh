package list

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/blacknon/lssh/conf"
	termbox "github.com/nsf/termbox-go"
)

// toggle select line (multi select)
func toggleList(selectedList []string, newLine string) (toggledSelectedList []string) {
	addFlag := true
	for _, selectedLine := range selectedList {
		if selectedLine != newLine {
			toggledSelectedList = append(toggledSelectedList, selectedLine)
		} else {
			addFlag = false
		}
	}
	if addFlag == true {
		toggledSelectedList = append(toggledSelectedList, newLine)
	}
	return
}

func allToggle(allFlag bool, selectedList []string, addList []string) (allSelectedList []string) {
	// selectedList in allSelectedList
	for _, selectedLine := range selectedList {
		allSelectedList = append(allSelectedList, selectedLine)
	}

	// allFlag is False
	if allFlag == false {
		for _, addLine := range addList {
			addData := strings.Fields(addLine)[0]
			allSelectedList = append(allSelectedList, addData)
		}
		return
	} else {
		for _, addLine := range addList {
			addData := strings.Fields(addLine)[0]
			allSelectedList = toggleList(allSelectedList, addData)
		}
		return
	}
}

// Create View List Data (use text/tabwriter)
func getListData(serverNameList []string, serverList conf.Config) (listData []string) {
	buffer := &bytes.Buffer{}
	tabWriterBuffer := new(tabwriter.Writer)
	tabWriterBuffer.Init(buffer, 0, 4, 8, ' ', 0)
	fmt.Fprintln(tabWriterBuffer, "ServerName \tConnect Infomation \tNote \t")

	// Create list table
	for _, key := range serverNameList {
		serverName := key
		connectInfomation := serverList.Server[key].User + "@" + serverList.Server[key].Addr
		serverNote := serverList.Server[key].Note

		fmt.Fprintln(tabWriterBuffer, serverName+"\t"+connectInfomation+"\t"+serverNote)
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

func (l *ListInfo) View() (lineName []string) {
	if err := termbox.Init(); err != nil {
		panic(err)
	}

	lineName = pollEvent(l.NameList, l.MultiFlag, l.DataList)
	return lineName
}
