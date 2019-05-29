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
	shellConf := r.Conf.Shell //shell config

	// create new shell struct
	s := new(shell)

	// create ssh shell connects
	conns := r.createConn()
	s.CreateConn(conns)

	// set prompt templete
	s.PROMPT = shellConf.Prompt
	s.OPROMPT = shellConf.OPrompt

	// set signal
	s.Signal = make(chan os.Signal)
	signal.Notify(s.Signal, syscall.SIGTERM, syscall.SIGINT)

	// create prompt
	shellPrompt, _ := s.CreatePrompt()

	// create new go-prompt
	p := prompt.New(
		s.Executor,
		s.Completer,
		prompt.OptionPrefix(shellPrompt),
		prompt.OptionLivePrefix(s.CreatePrompt),
		prompt.OptionInputTextColor(prompt.Green),
		prompt.OptionPrefixTextColor(prompt.Blue),
		prompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator), // test
	)

	// run go-prompt
	p.Run()
}

type shell struct {
	Signal   chan os.Signal
	Connects []*shellConn
	PROMPT   string
	OPROMPT  string
	Count    int
}

var (
	defaultPrompt  = "[$n]  >>>> " // Default PROMPT
	defaultOPrompt = "[$h][$n] > " // Default OPROMPT
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
			sc.StdoutData = new(bytes.Buffer)
			sc.StderrData = new(bytes.Buffer)

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
	p = strings.Replace(p, "$n", strconv.Itoa(s.Count), -1)
	p = strings.Replace(p, "$h", hostname, -1)
	p = strings.Replace(p, "$u", username, -1)
	p = strings.Replace(p, "$l", pwd, -1)

	return p, true
}

// @TODO: ユーザ名等も指定できるよう、指定やConfigの受け取り方を考える(優先度B)
// create shell output prompt
func (s *shell) CreateOPrompt(server string) (op string) {
	op = s.OPROMPT
	if op == "" {
		op = defaultOPrompt
	}

	// replace variable value
	op = strings.Replace(op, "$n", strconv.Itoa(s.Count), -1)
	op = strings.Replace(op, "$h", server, -1)

	return
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
				con.Kill(isExit)
			}

			return
		case <-isSignalExit:
			return
		}

	}(s.Signal, isSignalExit, s.Connects)

wait:
	for {
		for _, c := range s.Connects {
			s.outputData(c.Server, c.StdoutData)
			s.outputData(c.Server, c.StderrData)
		}

		select {
		case <-isFinished:
			time.Sleep(10 * time.Millisecond)
			break wait
		case <-time.After(10 * time.Millisecond):
			continue
		}
	}

	for _, c := range s.Connects {
		s.outputData(c.Server, c.StdoutData)
		s.outputData(c.Server, c.StderrData)
	}

	fmt.Fprintf(os.Stderr, "\n---\n%s\n", "Command exit. Please input Enter.")

	isInputExit <- true

	s.Count += 1
	return
}

// @TODO: サーバ名の幅をあわせるようにする
// @TODO: 可能であれば、run_cmdの出力関数とマージすることを検討
func (s *shell) outputData(server string, output *bytes.Buffer) {
	// Create output prompt
	op := s.CreateOPrompt(server)

	for {
		if output.Len() > 0 {
			line, err := output.ReadBytes('\n')
			str := string(line)
			str = strings.TrimRight(str, "\n")
			fmt.Printf("%s %s\n", op, str)
			if err == io.EOF {
				continue
			}
		} else {
			break
		}
	}
}
