package ssh

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/c-bata/go-prompt"
)

// variable
var (
	defaultPrompt  = "[${COUNT}] <<< "          // Default PROMPT
	defaultOPrompt = "[${SERVER}][${COUNT}] > " // Default OPROMPT
)

func (r *Run) pshell() {
	// print header
	fmt.Println("Start lssh-shell...")
	r.printSelectServer()

	// read shell config
	config := r.Conf.Shell

	// overwrite default value config.Prompt
	if config.Prompt == "" {
		config.Prompt = defaultPrompt
	}

	// overwrite default value config.OPrompt
	if config.OPrompt == "" {
		config.OPrompt = defaultOPrompt
	}

	// run pre cmd
	runCmdLocal(config.PreCmd)
	defer runCmdLocal(config.PostCmd)

	// create new shell struct
	p := &pshell{
		Signal:      make(chan os.Signal),
		ServerList:  r.ServerList,
		PROMPT:      config.Prompt,
		OPROMPT:     config.OPrompt,
		ExecHistory: map[int]string{},
	}

	// set signal
	signal.Notify(s.Signal, syscall.SIGTERM, syscall.SIGINT)

	fmt.Println("now not work...")
}

// pshell strcut
type pshell struct {
	Signal      chan os.Signal
	ServerList  []string
	Connects    []*shellConn
	PROMPT      string
	OPROMPT     string
	HistoryFile string
	Count       int
	ExecHistory map[int]string
	Complete    []prompt.Suggest
}
