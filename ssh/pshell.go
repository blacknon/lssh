package ssh

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/blacknon/go-sshlib"
	"github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
)

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

// variable
var (
	defaultPrompt      = "[${COUNT}] <<< "          // Default PROMPT
	defaultOPrompt     = "[${SERVER}][${COUNT}] > " // Default OPROMPT
	defaultHistoryFile = "~/.lssh_history"          // Default Parallel shell history file
)

func (r *Run) pshell() (err error) {
	// print header
	fmt.Println("Start parallel-shell...")
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
	ps := &PShell{
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
	signal.Notify(ps.Signal, syscall.SIGTERM, syscall.SIGINT)

	// old history list
	var historyCommand []string
	oldHistory, err := ps.GetHistory()
	if err == nil {
		for _, h := range oldHistory {
			historyCommand = append(historyCommand, h.Command)
		}
	}

	// create complete data
	ps.GetCompleteData()

	// create prompt
	pshellPrompt, _ := p.CreatePrompt()

	// create go-prompt
	p := prompt.New(
		ps.Executor,
		ps.Completer,
		prompt.OptionHistory(historyCommand),
		prompt.OptionPrefix(pshellPrompt),
		prompt.OptionLivePrefix(ps.CreatePrompt),
		prompt.OptionInputTextColor(prompt.Green),
		prompt.OptionPrefixTextColor(prompt.Blue),
		prompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator), // test
	)

	// start go-prompt
	p.Run()

	return
}

// CreatePrompt is create shell prompt.
// default value is `[${COUNT}] <<< `
func (ps *PShell) CreatePrompt() (p string, result bool) {
	// set prompt templete (from conf)
	p = ps.PROMPT
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

// Executor run ssh command in parallel-shell.
//
func (ps *PShell) Executor(cmd string) {
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
		// run post command
		runCmdLocal(ps.PostCmd)

		// put history and exit
		ps.PutHistory(cmd)
		os.Exit(0)

	// clear
	case cmd == "clear":
		fmt.Printf("\033[H\033[2J")
		ps.PutHistory(cmd)
		return

	// history
	case cmd == "history":
		ps.localCmd_history()
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

		ps.localCmd_out(num)
		fmt.Println()
		return
	}

	// put history
	ps.PutHistory(cmd)

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
	for _, c := range ps.Connects {
		// TODO(blacknon): エラーハンドリングする
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
	for _, c := range ps.Connects {
		c.Count = ps.Count
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
	}(ps.Signal, isSignalExit, ps.Connects)

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
	ps.ExecHistory[s.Count] = cmd
	ps.Count += 1
	return
}

// Completer lssh-shell complete function
func (ps *pShell) Completer(t prompt.Document) []prompt.Suggest {
	// TODO(blacknon): とりあえず値を仮置き。後で以下の処理を追加する(優先度A)
	//        - compgen(confで補完用の結果を取得するためのコマンドは指定可能にする)での補完結果の定期取得処理(+補完の取得用ローカルコマンドの追加)
	//        - compgenの結果をStructに保持させる
	//        - Structに保持されている補完内容をベースにCompleteの結果を返す
	//        - 何も入力していない場合は非表示にさせたい
	//        - ファイルについても対応させたい
	//        - ファイルやコマンドなど、状況に応じて補完対象を変えるにはやはり構文解析が必要になってくる。Parserを実装するまではコマンドのみ対応。
	//        	参考: https://github.com/c-bata/kube-prompt/blob/2276d167e2e693164c5980427a6809058a235c95/kube/completer.go

	// local command suggest
	localCmdSuggest := []prompt.Suggest{
		{Text: "exit", Description: "exit lssh shell"},
		{Text: "quit", Description: "exit lssh shell"},
		{Text: "clear", Description: "clear screen"},
		{Text: "history", Description: "show history"},
		{Text: "%out", Description: "%out [num], show history result."},

		// outのリストを出力ためのローカルコマンド
		// {Text: "%outlist", Description: "%outlist, show history result list."},

		// outの出力でdiffをするためのローカルコマンド。すべての出力と比較するのはあまりに辛いと思われるため、最初の出力との比較、といった方式で対応するのが良いか？？
		// {Text: "%diff", Description: "%diff [num], show history result list."},

		// outの出力でユニークな出力だけを表示するコマンド
		// {Text: "%unique", Description: "%unique [num], show history result list."},

		// outの出力で重複している出力だけを表示するコマンド
		// {Text: "%duplicate", Description: "%duplicate [num], show history result list."},

	}

	// get complete data
	c := ps.Complete
	c = append(ps, localCmdSuggest...)

	return prompt.FilterHasPrefix(c, t.GetWordBeforeCursor(), false)
}

// GetCompleteData get command list remote machine.
func (ps *pShell) GetCompleteData() {
	// bash complete command
	compCmd := []string{"compgen", "-c"}

	// TODO(blacknon):
	// - 重複データの排除
	// - 構文解析して、ファイルの補完処理も行わせる
	//   - 引数にコマンドorファイルの種別を渡すようにする
	// - 補完コマンドをconfigでオプションとして指定できるようにする
	//   - あまり無いだろうけど、zshをリモートで使ってる場合なんかには指定(zshとかはデフォルトでcompgen使えないし)
	//   - ashの補完ってどうしてるんだ？？

	for _, c := range ps.Connects {
		buf := new(bytes.Buffer)
		session, _ := c.CreateSession()
		session.Stdout = buf
		c.RunCmd(session, compCmd)
		sc := bufio.NewScanner(buf)
		for sc.Scan() {
			suggest := prompt.Suggest{
				Text:        sc.Text(),
				Description: "Command. from:" + c.Server,
			}
			ps.Complete = append(ps.Complete, suggest)
		}
	}
}

// Run is exec command.
func (ps *pShell) Run(command string) {

}
