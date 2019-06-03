package ssh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
)

func (r *Run) shell() {
	// @TEST : Non Blocking I/O
	// fd := int(os.Stdin.Fd())
	// syscall.SetNonblock(fd, true)

	// @TODO: 接続できてない状態でもコンソールに入ってしまうので、そこの処理を書き換える

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

	// ServerList
	s.ServerList = r.ServerList

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

	// if can connect host not found...
	if len(s.Connects) == 0 {
		return
	}

	// history file
	s.HistoryFile = shellConf.HistoryFile

	// create prompt
	shellPrompt, _ := s.CreatePrompt()

	// create new go-prompt
	p := prompt.New(
		s.Executor,
		s.Completer,
		prompt.OptionHistory([]string{"ls -la", "pwd"}),
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
	Count       int
}

// variable
var (
	defaultPrompt  = "[${COUNT}] <<< "          // Default PROMPT
	defaultOPrompt = "[${SERVER}][${COUNT}] > " // Default OPROMPT
)

// Convert []*Connect to []*shellConn, and Connect ssh
func (s *shell) CreateConn(conns []*Connect) {
	isExit := make(chan bool)
	connectChan := make(chan *Connect)

	// create ssh connect
	for _, c := range conns {
		conn := c

		// Connect ssh (goroutine)
		go func() {
			// Connect ssh
			err := conn.CreateClient()

			// send exit channel
			isExit <- true

			// check error
			if err != nil {
				fmt.Fprintf(os.Stderr, "Cannot connect session %v, %v\n", conn.Server, err)
				return
			}

			// send ssh client
			connectChan <- conn
		}()
	}

	for i := 0; i < len(conns); i++ {
		<-isExit

		select {
		case c := <-connectChan:
			// create shellConn
			sc := new(shellConn)
			sc.Connect = c
			sc.ServerList = s.ServerList
			sc.StdoutData = new(bytes.Buffer)
			sc.StderrData = new(bytes.Buffer)
			sc.OutputPrompt = s.OPROMPT

			// append shellConn
			s.Connects = append(s.Connects, sc)
		case <-time.After(10 * time.Millisecond):
			continue
		}

	}
}

// @TODO: KeepAlive用のリクエスト送信用の関数。後で記述する。多分channelで終わらせてあげないとだめかも？？(優先度 E)
// func (s *shell) sendKeepAlive() {}

// create shell prompt
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

// run ssh command
// @TODO: 全体的に見直しが必須！
func (s *shell) Executor(cmd string) {
	// trim space
	cmd = strings.TrimSpace(cmd)

	// check local command
	// @TODO: 後でrun_shell_cmd.goに移してちゃんと作る
	switch cmd {
	case "":
		return
	case "exit", "quit":
		os.Exit(0)
	case "clear":
		fmt.Printf("\033[H\033[2J")
		return
	}

	// create chanel
	isExit := make(chan bool)
	// isKill := make(chan bool)
	isFinished := make(chan bool)
	isInputExit := make(chan bool)
	isSignalExit := make(chan bool)

	// defer close channel
	defer close(isExit)
	// defer close(isKill)
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
		case <-time.After(10 * time.Millisecond):
			continue
		}
	}

	fmt.Fprintf(os.Stderr, "\n---\n%s\n", "Command exit. Please input Enter.")
	isInputExit <- true
	s.Count += 1
	return
}
