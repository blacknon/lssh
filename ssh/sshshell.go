package ssh

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/blacknon/lssh/common"
	"github.com/c-bata/go-prompt"
	"golang.org/x/crypto/ssh"
)

var (
	defaultPrompt  = "[$n]  >>>> " // Default PROMPT
	defaultOPrompt = "[$h][$n] > " // Default OPROMPT
)

type shellConn struct {
	*Connect
	Session    *ssh.Session
	StdoutData *bytes.Buffer
	StderrData *bytes.Buffer
}

// @TODO: 変数名その他いろいろと見直しをする！！
//        ローカルのコマンドとパイプでつなげるような処理を実装する予定なので、Stdin、Stdout等の扱いを分離して扱いやすくする
func (c *shellConn) SshShellCmdRun(cmd string, isExit chan<- bool) (err error) {
	c.Session.Stdout = c.StdoutData
	c.Session.Stderr = c.StderrData

	c.Session.Run(cmd)

	isExit <- true
	return
}

func (c *shellConn) Kill(isExit chan<- bool) (err error) {
	time.Sleep(10 * time.Millisecond)
	c.Session.Signal(ssh.SIGINT)
	err = c.Session.Close()
	return
}

type shell struct {
	Signal   chan os.Signal
	Connects []*shellConn
	PROMPT   string
	OPROMPT  string
	Count    int
}

// Convert []*Connect to []*shellConn, and Connect ssh
func (s *shell) CreateConn(conns []*Connect) {
	for _, c := range conns {
		sc := new(shellConn)
		sc.Connect = c

		// Connect ssh
		// @TODO: 接続をパラレルで実行するよう、Connectをgoroutineで行うようにする
		err := sc.CreateClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot connect session %v, %v\n", sc.Server, err)
			continue
		}

		sc.StdoutData = new(bytes.Buffer)
		sc.StderrData = new(bytes.Buffer)

		s.Connects = append(s.Connects, sc)
	}
}

// @TODO: KeepAlive用のリクエスト送信用の関数。後で記述する。多分channelで終わらせてあげないとだめかも？？
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

// shell complete function
// @TODO: とりあえず値を仮置き。後で以下の処理を追加する
//        ・ compgen(confで補完用の結果を取得するためのコマンドは指定可能にする)での補完結果の定期取得処理(+補完の取得用ローカルコマンドの追加)
//        ・ compgenの結果をStructに保持させる
//        ・ Structに保持されている補完内容をベースにCompleteの結果を返す
func (s *shell) Completer(t prompt.Document) []prompt.Suggest {
	ps := []prompt.Suggest{
		{Text: "test-suggest"},
	}
	return prompt.FilterHasPrefix(ps, t.GetWordBeforeCursor(), true)
}

// run ssh command
// @TODO: 全体的に見直しが必須！
func (s *shell) Executor(cmd string) {
	// delete head space
	cmd = common.RegexRep(string(cmd), "", "^ *")

	// check local command
	// @TODO: 後でsshshell_cmd.goに移してちゃんと作る
	switch cmd {
	case "":
		return
	case "exit":
		return
	case "clear":
		fmt.Printf("\033[H\033[2J")
		return
	}

	// create chanel
	// @TODO: 後で見直し
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
	// @TODO: 後でcommand runと同じ関数に統合する
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

	// @TODO: 出力が完了するまでに処理が終わってしまい待ちが発生することがあるので、違うとこで出力させる
	fmt.Fprintf(os.Stderr, "\n\n%s\n", "run exit. input Enter.")

	// isSignalExit <- true
	isInputExit <- true

	s.Count += 1
	return
}

// @TODO: 後でcommand runの関数と統合するかなんかする
func (s *shell) outputData(server string, output *bytes.Buffer) {
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

// @TODO: 後でcommand runの関数と統合する
func pushInput(isExit <-chan bool, writer io.Writer) {
	rd := bufio.NewReader(os.Stdin)
loop:
	for {
		data, _ := rd.ReadBytes('\n')
		if len(data) > 0 {
			writer.Write(data)
		}

		select {
		case <-isExit:
			break loop
		case <-time.After(10 * time.Millisecond):
			continue
		}
	}
}
