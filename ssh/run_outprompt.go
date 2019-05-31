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
	Conf       map[string]conf.ServerConfig
	AutoColor  bool
}

// @TODO:
//     - Text templeteでの処理に切り替え
//     - Structを作って処理させるように切り替える
// Create OPROMPT
//     - $n ... Count
//     - $s ... Server
//     - $h ... Addr
//     - $u ... User
//     - $p ... Port
func (o *Output) Create(server string) {
	o.server = server
	data := o.Conf[server]

	// get max length at server name
	length := common.GetMaxLength(o.ServerList)

	// get color num
	n := common.GetOrderNumber(server, o.ServerList)
	colorServerName := outColorStrings(n, server)

	// set templete
	p := o.Templete

	// server info
	p = strings.Replace(p, "$s", fmt.Sprintf("%-*s", length, colorServerName), -1)
	p = strings.Replace(p, "$h", data.Addr, -1)
	p = strings.Replace(p, "$u", data.User, -1)
	p = strings.Replace(p, "$p", data.Port, -1)

	o.prompt = p
}

// update variable value
func (o *Output) GetPrompt() (up string) {
	// replace variable value
	up = strings.Replace(o.prompt, "$n", strconv.Itoa(0), -1)

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
