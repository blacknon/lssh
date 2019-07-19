package ssh

import (
	"fmt"
)

// variable
// var (
// 	defaultPrompt  = "[${COUNT}] <<< "          // Default PROMPT
// 	defaultOPrompt = "[${SERVER}][${COUNT}] > " // Default OPROMPT
// )

// strcut
// type pshell struct {
// 	Signal      chan os.Signal
// 	ServerList  []string
// 	Connects    []*shellConn
// 	PROMPT      string
// 	OPROMPT     string
// 	HistoryFile string
// 	PreCmd      string
// 	PostCmd     string
// 	Count       int
// 	ExecHistory map[int]string
// 	Complete    []prompt.Suggest
// }

func (r *Run) pshell() {
	// // print header
	// fmt.Println("Start lssh-shell...")
	// r.printSelectServer()

	// // read shell config
	// sConf := r.Conf.Shell

	// // run pre cmd
	// runCmdLocal(sConf.PreCmd)
	// defer runCmdLocal(sConf.PostCmd)

	// // create new shell struct
	// p := new(pshell)

	fmt.Println("not work...")

}
