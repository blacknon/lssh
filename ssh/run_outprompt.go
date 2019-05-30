package ssh

import (
	"strconv"
	"strings"

	"github.com/blacknon/lssh/conf"
)

// Output Prompt
type OPrompt struct {
	Templete   string
	Count      int
	ServerList []string
	Conf       []conf.ServerConfig
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
func (op *OPrompt) Create(server string) (p string) {
	// server info
	p = strings.Replace(p, "$s", server, -1)
	// p = strings.Replace(p, "$h", data.Addr, -1)
	// p = strings.Replace(p, "$u", data.User, -1)
	// p = strings.Replace(p, "$p", data.Port, -1)

	return
}

// update variable value
func (op *OPrompt) Update(p string) (up string) {
	// replace variable value
	up = strings.Replace(p, "$n", strconv.Itoa(0), -1)

	return
}
