package ssh

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/blacknon/go-sshlib"
	"github.com/c-bata/go-prompt"
)

// variable
var (
	defaultPrompt      = "[${COUNT}] <<< "          // Default PROMPT
	defaultOPrompt     = "[${SERVER}][${COUNT}] > " // Default OPROMPT
	defaultHistoryFile = "~/.lssh_history"          // Default Parallel shell history file
)

func (r *Run) pshell() (err error) {
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

	// overwrite default parallel shell history file
	if config.HistoryFile == "" {
		config.HistoryFile = defaultHistoryFile
	}

	// run pre cmd
	runCmdLocal(config.PreCmd)
	defer runCmdLocal(config.PostCmd)

	// Connect
	var cons []*sshlib.Connect
	for _, server := range r.ServerList {
		con, err := r.createSshConnect(server)
		if err != nil {
			log.Println(err)
		}
		cons = append(cons, con)
	}

	// count sshlib.Connect.
	if len(cons) == 0 {
		return
	}

	// create new shell struct
	p := &PShell{
		Signal:      make(chan os.Signal),
		Count:       0,
		ServerList:  r.ServerList,
		Connects:    cons,
		PROMPT:      config.Prompt,
		OPROMPT:     config.OPrompt,
		History:     map[int]map[string]PShellHistory{},
		HistoryFile: config.HistoryFile,
	}

	// set signal
	signal.Notify(p.Signal, syscall.SIGTERM, syscall.SIGINT)

	// Debug
	fmt.Println(p)

	return
}

// Pshell is Parallel-Shell struct
type PShell struct {
	Signal      chan os.Signal
	Count       int
	ServerList  []string
	Connects    []*sshlib.Connect
	PROMPT      string
	OPROMPT     string
	History     map[int]map[string]PShellHistory
	HistoryFile string
	Complete    []prompt.Suggest
}

type PShellHistory struct{}
