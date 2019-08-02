package ssh

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/blacknon/go-sshlib"
	"github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
)

// Pshell is Parallel-Shell struct
type pShell struct {
	Signal      chan os.Signal
	Count       int
	ServerList  []string
	Connects    []*psConnect
	PROMPT      string
	History     map[int]map[string]*pShellHistory
	HistoryFile string
	Complete    []prompt.Suggest
}

// psConnect is pShell connect struct.
type psConnect struct {
	Name   string
	Output *Output
	*sshlib.Connect
}

// variable
var (
	// Default PROMPT
	defaultPrompt = "[${COUNT}] <<< "

	// Default OPROMPT
	defaultOPrompt = "[${SERVER}][${COUNT}] > "

	// Default Parallel shell history file
	defaultHistoryFile = "~/.lssh_history"
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
	var cons []*psConnect
	for _, server := range r.ServerList {
		// Create *sshlib.Connect
		con, err := r.createSshConnect(server)
		if err != nil {
			log.Println(err)
			continue
		}

		// TTY enable
		con.TTY = true

		// Create Output
		o := &Output{
			Templete:   config.OPrompt,
			ServerList: r.ServerList,
			Conf:       r.Conf.Server[server],
			AutoColor:  true,
		}

		// Create output prompt
		o.Create(server)

		psCon := &psConnect{
			Name:    server,
			Output:  o,
			Connect: con,
		}
		cons = append(cons, psCon)
	}

	// count sshlib.Connect.
	if len(cons) == 0 {
		return
	}

	// create new shell struct
	ps := &pShell{
		Signal:      make(chan os.Signal),
		ServerList:  r.ServerList,
		Connects:    cons,
		PROMPT:      config.Prompt,
		History:     map[int]map[string]*pShellHistory{},
		HistoryFile: config.HistoryFile,
	}

	// set signal
	signal.Notify(ps.Signal, syscall.SIGTERM, syscall.SIGINT)

	// old history list
	var historyCommand []string
	oldHistory, err := ps.GetHistoryFromFile()
	if err == nil {
		for _, h := range oldHistory {
			historyCommand = append(historyCommand, h.Command)
		}
	}

	// create complete data
	ps.GetCompleteData()

	// create prompt
	pShellPrompt, _ := ps.CreatePrompt()

	// create go-prompt
	p := prompt.New(
		ps.Executor,
		ps.Completer,
		prompt.OptionHistory(historyCommand),
		prompt.OptionPrefix(pShellPrompt),
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
func (ps *pShell) CreatePrompt() (p string, result bool) {
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
	p = strings.Replace(p, "${COUNT}", strconv.Itoa(ps.Count), -1)
	p = strings.Replace(p, "${HOSTNAME}", hostname, -1)
	p = strings.Replace(p, "${USER}", username, -1)
	p = strings.Replace(p, "${PWD}", pwd, -1)

	return p, true
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

	// TODO(blacknon):
	//        - t.GetWordBeforeCursor() などで前の文字までは取得できるので、その文字列に応じて補完を返すかどうかを対応する。
	//        - パイプラインを区切る際、複数のセパレータで区切れるか調査が必要(|や;の他、' 'や||、&&など)。
	//          (多分、位置情報と組み合わせてコマンドラインを取得して、位置より前の情報からセパレートして処理してやればどうにかなりそう。)

	// build in command suggest
	localCmdSuggest := []prompt.Suggest{
		{Text: "exit", Description: "exit lssh shell"},
		{Text: "quit", Description: "exit lssh shell"},
		{Text: "clear", Description: "clear screen"},
		{Text: "%history", Description: "show history"},
		{Text: "%out", Description: "%out [num], show history result."},
		// {Text: "%outlist", Description: "%outlist, show history result list."}, // outのリストを出力ためのローカルコマンド

		// outの出力でdiffをするためのローカルコマンド。すべての出力と比較するのはあまりに辛いと思われるため、最初の出力との比較、といった方式で対応するのが良いか？？
		// {Text: "%diff", Description: "%diff [num], show history result list."},

		// outの出力でユニークな出力だけを表示するコマンド
		// {Text: "%unique", Description: "%unique [num], show history result list."},

		// outの出力で重複している出力だけを表示するコマンド
		// {Text: "%duplicate", Description: "%duplicate [num], show history result list."},
	}

	// get complete data
	c := ps.Complete
	c = append(c, localCmdSuggest...)

	return prompt.FilterHasPrefix(c, t.GetWordBeforeCursor(), false)
}

// GetCompleteData get command list remote machine.
func (ps *pShell) GetCompleteData() {
	// TODO(blacknon):
	//   - 構文解析して、ファイルの補完処理も行わせる
	//     - 引数にコマンドorファイルの種別を渡すようにする
	//   - 補完コマンドをconfigでオプションとして指定できるようにする
	//     - あまり無いだろうけど、bash以外をリモートで使ってる場合など(ashとかzsh(レア)など)

	// bash complete command. use `compgen`.
	compCmd := []string{"compgen", "-c"}
	command := strings.Join(compCmd, " ")

	// command map
	cmdMap := map[string][]string{}

	// append command to cmdMap
	for _, c := range ps.Connects {
		// Create buffer
		buf := new(bytes.Buffer)

		// Create session, and output to buffer
		session, _ := c.CreateSession()
		session.Stdout = buf

		// Run get complete command
		session.Run(command)

		// Scan and put completed command to map.
		sc := bufio.NewScanner(buf)
		for sc.Scan() {
			cmdMap[sc.Text()] = append(cmdMap[sc.Text()], c.Name)
		}
	}

	// cmdMap to suggest
	for cmd, hosts := range cmdMap {
		// join hosts
		h := strings.Join(hosts, ",")

		// create suggest
		suggest := prompt.Suggest{
			Text:        cmd,
			Description: "Command. from:" + h,
		}

		// append ps.Complete
		ps.Complete = append(ps.Complete, suggest)
	}
}

// Run is exec command at remote machine.
// TODO(blacknon):
//     - 標準入出力をパイプ経由でやり取りできるよう、汎用性を考慮する
//     - 入出力の指定とoutputへのデータの送信処理を分離する必要がある？？
//     - 残すにしても名前変えないとわかりにくいし辛いことになりそう。
//         - `ExecuteRemoteCmd`とかかな
//         - 入出力含め、リファクタが必要
func (ps *pShell) remoteRun(command string) {
	// Create History
	ps.History[ps.Count] = map[string]*pShellHistory{}

	// create chanel
	finished := make(chan bool)    // Run Command finish channel
	input := make(chan io.Writer)  // Get io.Writer at input channel
	exitInput := make(chan bool)   // Input finish channel
	exitSignal := make(chan bool)  // Send kill signal finish channel
	exitHistory := make(chan bool) // Put History finish channel

	// create []io.Writer after in MultiWriter
	var writers []io.Writer

	// for connect and run
	m := new(sync.Mutex)
	for _, fc := range ps.Connects {
		// set variable c
		// NOTE: Variables need to be assigned separately for processing by goroutine.
		c := fc

		// Get output data channel
		output := make(chan []byte)
		// defer close(output)

		// Set count num
		c.Output.Count = ps.Count

		// Create output buffer, and MultiWriter
		buf := new(bytes.Buffer)
		omw := io.MultiWriter(os.Stdout, buf)
		c.Output.OutputWriter = omw

		// put result
		go func() {
			m.Lock()
			ps.PutHistoryResult(c.Name, command, buf, exitHistory)
			m.Unlock()
		}()

		// Run command
		go func() {
			c.CmdWriter(command, output, input)
			finished <- true
		}()

		// Get input(io.Writer), add MultiWriter
		w := <-input
		writers = append(writers, w)

		// run print Output
		go func() {
			printOutput(c.Output, output)
		}()
	}

	// create and run input writer
	mw := io.MultiWriter(writers...)
	go pushInput(exitInput, mw)

	// send kill signal function
	go ps.pushKillSignal(exitSignal, ps.Connects)

	// wait finished channel
	for i := 0; i < len(ps.Connects); i++ {
		select {
		case <-finished:
		}
	}

	// Exit check signal
	exitSignal <- true

	// wait time (0.500 sec)
	time.Sleep(100 * time.Millisecond)

	// Exit Messages
	// Because it is Blocking.IO, you can not finish Input without input from the user.
	fmt.Fprintf(os.Stderr, "\n---\n%s\n", "Command exit. Please input Enter.")

	// Exit input
	exitInput <- true

	// Exit history
	for i := 0; i < len(ps.Connects); i++ {
		exitHistory <- true
	}

	// Add count
	ps.Count += 1
}

// pushSignal is send kill signal to session.
func (ps *pShell) pushKillSignal(exitSig chan bool, conns []*psConnect) (err error) {
	i := 0
	for {
		select {
		case <-ps.Signal:
			if i == 0 {
				for _, c := range conns {
					// send kill
					c.Kill()
				}
				i = 1
			}
		case <-exitSig:
			return
		}
	}
	return
}