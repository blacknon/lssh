package ssh

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/blacknon/lssh/common"
	"github.com/blacknon/lssh/conf"
)

// Output Prompt
type Output struct {
	Templete   string
	prompt     string
	server     string
	Count      int
	ServerList []string
	Conf       conf.ServerConfig
	AutoColor  bool
}

// @TODO:
//     - Text templeteでの処理に切り替え
//     - Structを作って処理させるように切り替える
// Create OPROMPT
//     - ${COUNT}  ... Count
//     - ${SERVER} ... Server
//     - ${ADDR}   ... Addr
//     - ${USER}   ... User
//     - ${PORT}   ... Port
func (o *Output) Create(server string) {
	o.server = server

	// get max length at server name
	length := common.GetMaxLength(o.ServerList)
	addL := length - len(server)

	// get color num
	n := common.GetOrderNumber(server, o.ServerList)
	colorServerName := outColorStrings(n, server)

	// set templete
	p := o.Templete

	// server info
	p = strings.Replace(p, "${SERVER}", fmt.Sprintf("%-*s", len(colorServerName)+addL, colorServerName), -1)
	p = strings.Replace(p, "${ADDR}", o.Conf.Addr, -1)
	p = strings.Replace(p, "${USER}", o.Conf.User, -1)
	p = strings.Replace(p, "${PORT}", o.Conf.Port, -1)

	o.prompt = p
}

// update variable value
func (o *Output) GetPrompt() (up string) {
	// replace variable value
	up = strings.Replace(o.prompt, "${COUNT}", strconv.Itoa(o.Count), -1)
	return
}

func printOutput(o *Output, output chan []byte) {
	// print output
	for data := range output {
		str := strings.TrimRight(string(data), "\n")
		if len(o.ServerList) > 1 {
			oPrompt := o.GetPrompt()
			fmt.Printf("%s %s\n", oPrompt, str)
		} else {
			fmt.Printf("%s\n", str)
		}
	}
}

func outColorStrings(num int, inStrings string) (str string) {
	// 1=Red,2=Yellow,3=Blue,4=Magenta,0=Cyan
	color := 31 + num%5

	str = fmt.Sprintf("\x1b[%dm%s\x1b[0m", color, inStrings)
	return
}
