package ssh

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
)

// run lssh-shell
func (r *Run) shell() {
	// print header
	fmt.Println("Start lssh shell...")
	r.printSelectServer()

	// print newline
	fmt.Println()

	// read shell config
	shellConf := r.Conf.Shell

	// run pre cmd
	runCmdLocal(shellConf.PreCmd)
	defer runCmdLocal(shellConf.PostCmd)

	// create new shell struct
	s := new(shell)

	// ExecHistory
	s.ExecHistory = map[int]string{}

	// ServerList
	s.ServerList = r.ServerList

	// pre/post command
	s.PreCmd = shellConf.PreCmd
	s.PostCmd = shellConf.PostCmd

	// set prompt templete
	s.PROMPT = shellConf.Prompt
	s.OPROMPT = shellConf.OPrompt
	if s.OPROMPT == "" {
		s.OPROMPT = defaultOPrompt
	}

	// set signal
	s.Signal = make(chan os.Signal)
	signal.Notify(s.Signal, syscall.SIGTERM, syscall.SIGINT)

	// create ssh shell connects
	conns := r.createConn()
	s.CreateConn(conns)

	// TODO(blacknon): keep aliveの送信処理を入れる

	// if can connect host not found...
	if len(s.Connects) == 0 {
		return
	}

	// history file
	s.HistoryFile = shellConf.HistoryFile
	if s.HistoryFile == "" {
		s.HistoryFile = "~/.lssh_history"
	}

	// create history list
	var histCmdList []string
	histList, err := s.GetHistory()
	if err == nil {
		for _, hist := range histList {
			histCmdList = append(histCmdList, hist.Command)
		}
	}

	// create complete data
	s.GetCompleteData()

	// create prompt
	shellPrompt, _ := s.CreatePrompt()

	// create new go-prompt
	p := prompt.New(
		s.Executor,
		s.Completer,
		prompt.OptionHistory(histCmdList),
		prompt.OptionPrefix(shellPrompt),
		prompt.OptionLivePrefix(s.CreatePrompt),
		prompt.OptionInputTextColor(prompt.Green),
		prompt.OptionPrefixTextColor(prompt.Blue),
		prompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator), // test
	)

	// run go-prompt
	p.Run()
}

// strcut
type shell struct {
	Signal      chan os.Signal
	ServerList  []string
	Connects    []*shellConn
	PROMPT      string
	OPROMPT     string
	HistoryFile string
	PreCmd      string
	PostCmd     string
	Count       int
	ExecHistory map[int]string
	Complete    []prompt.Suggest
}

// variable
var (
	defaultPrompt  = "[${COUNT}] <<< "          // Default PROMPT
	defaultOPrompt = "[${SERVER}][${COUNT}] > " // Default OPROMPT
)

// CreatePrompt is create shell prompt. default value `[${COUNT}] <<< `
func (s *shell) CreatePrompt() (p string, result bool) {
	// set prompt templete (from conf)
	p = s.PROMPT
	if p == "" {
		p = defaultPrompt
	}

	// Get env
	hostname, _ := os.Hostname()
	username := os.Getenv("USER")
	pwd := os.Getenv("PWD")

	// replace variable value
	p = strings.Replace(p, "${COUNT}", strconv.Itoa(s.Count), -1)
	p = strings.Replace(p, "${HOSTNAME}", hostname, -1)
	p = strings.Replace(p, "${USER}", username, -1)
	p = strings.Replace(p, "${PWD}", pwd, -1)

	return p, true
}

// Executor run ssh command in lssh-shell
func (s *shell) Executor(cmd string) {
	// trim space
	cmd = strings.TrimSpace(cmd)

	// local command regex
	localCmdRegex_out := regexp.MustCompile(`^%out [0-9]+$`)

	// check local command
	switch {
	// only enter(Ctrl + M)
	case cmd == "":
		return

	// exit or quit
	case cmd == "exit", cmd == "quit":
		runCmdLocal(s.PostCmd)
		s.PutHistory(cmd)
		os.Exit(0)

	// clear
	case cmd == "clear":
		fmt.Printf("\033[H\033[2J")
		s.PutHistory(cmd)
		return

	// history
	case cmd == "history":
		s.localCmd_history()
		return

	// !out [num]
	case localCmdRegex_out.MatchString(cmd):
		cmdSlice := strings.SplitN(cmd, " ", 2)
		num, err := strconv.Atoi(cmdSlice[1])
		if err != nil {
			return
		}

		if num >= s.Count {
			return
		}

		s.localCmd_out(num)
		fmt.Println()
		return
	}

	// put history
	s.PutHistory(cmd)

	// create chanel
	isExit := make(chan bool)
	isFinished := make(chan bool)
	isInputExit := make(chan bool)
	isSignalExit := make(chan bool)

	// defer close channel
	defer close(isExit)
	defer close(isFinished)
	defer close(isInputExit)
	defer close(isSignalExit)

	// create writers
	writers := []io.Writer{}
	for _, c := range s.Connects {
		// @TODO: エラーハンドリングする
		session, _ := c.CreateSession()
		c.Session = session

		w, _ := c.Session.StdinPipe()
		writers = append(writers, w)
	}

	// create MultiWriter
	multiWriter := io.MultiWriter(writers...)

	// Run input goroutine
	go pushInput(isInputExit, multiWriter)

	// run command
	for _, c := range s.Connects {
		c.Count = s.Count
		go c.SshShellCmdRun(cmd, isExit)
	}

	// get command exit
	go func() {
		// get command exit
		for i := 0; i < len(s.Connects); i++ {
			<-isExit
		}
		isFinished <- true
	}()

	// get signal
	go func(isSignal chan os.Signal, isSignalExit chan bool, connect []*shellConn) {
		select {
		case <-isSignal:
			for _, con := range connect {
				con.Kill()
			}
			return
		case <-isSignalExit:
			return
		}
	}(s.Signal, isSignalExit, s.Connects)

wait:
	for {
		select {
		case <-isFinished:
			time.Sleep(10 * time.Millisecond)
			break wait
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}

	fmt.Fprintf(os.Stderr, "\n---\n%s\n", "Command exit. Please input Enter.")
	isInputExit <- true
	s.ExecHistory[s.Count] = cmd
	s.Count += 1
	return
}
