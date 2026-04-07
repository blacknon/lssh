// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package pshell

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/blacknon/go-sshlib"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/output"
	sshcmd "github.com/blacknon/lssh/internal/ssh"
	"github.com/c-bata/go-prompt"
)

// TODO(blacknon): 接続が切れた場合の再接続処理、および再接続ができなかった場合のsliceからの削除対応の追加(v0.8.0)
// TODO(blacknon): pShellのログ(実行コマンド及び出力結果)をログとしてファイルに記録する機能の追加(v0.7.1) => 任意のファイルを指定するように

// TODO(blacknon): petをうまいこと利用できるような仕組みを作る(v0.7.0)
// TODO(blacknon): parallel shellでkeybindや関数が使えるような仕組みを作る(どうやってやるかは不明だが…)(v0.7.1)
// TODO(blacknon): グループ化(`()`で囲んだりする)や三項演算子の対応(v0.7.1)
// TODO(blacknon): 環境変数などの仕組みを取り入れる。例えば、${SERVER}や${COUNT}などの変数をプロンプトやコマンドの中で使えるようにする(v0.7.1)

// shell is lsshell struct
type shell struct {
	Config        conf.ShellConfig
	Signal        chan os.Signal
	Run           *sshcmd.Run
	Count         int
	ServerList    []string
	Connects      []*sConnect
	PROMPT        string
	History       map[int]map[string]*shellHistory
	HistoryMu     *sync.Mutex
	HistoryFile   string
	latestCommand string
	currentConns  []*sConnect
	CmdComplete   []prompt.Suggest
	TargetCmdComp []prompt.Suggest
	TargetSrvComp []prompt.Suggest
	TargetCmdKey  string
	TargetSrvKey  string
	PathComplete  []prompt.Suggest
	Options       shellOption
}

// shellOption is optitons pshell.
// TODO(blacknon): つくる。: v0.7.1
type shellOption struct {
	// trueの場合、リモートマシンでパイプライン処理をする際にパイプ経由でもOPROMPTを付与して出力する
	// RemoteHeaderWithPipe bool

	// trueの場合、リモートマシンにキーインプットを送信しない
	// hogehoge

	// trueの場合、コマンドの補完処理を無効にする
	// DisableCommandComplete bool

	// trueの場合、PATHの補完処理を無効にする
	// DisableCommandComplete bool

	// local command実行時の結果をHistoryResultに記録しない(os.Stdoutに直接出す)
	LocalCommandNotRecordResult bool
}

// sConnect is shell connect struct.
type sConnect struct {
	Name   string
	Output *output.Output
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

func Shell(r *sshcmd.Run) (err error) {
	// print header
	fmt.Println("Start parallel-shell...")
	r.PrintSelectServer()

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
	execLocalCommand(config.PreCmd)
	defer execLocalCommand(config.PostCmd)

	// Connect
	// TODO: to change parallel
	var cons []*sConnect
	for _, server := range r.ServerList {
		// Create *sshlib.Connect
		con, err := r.CreateSshConnect(server)
		if err != nil {
			log.Println(err)
			continue
		}

		// TTY enable
		con.TTY = true

		forwardConf := r.PrepareParallelForwardConfig(server)
		if err := sshcmd.StartParallelForwards(con, forwardConf); err != nil {
			log.Println(err)
			if con.Client != nil {
				_ = con.Client.Close()
			}
			continue
		}

		// Create Output
		o := &output.Output{
			Templete:   config.OPrompt,
			ServerList: r.ServerList,
			Conf:       r.Conf.Server[server],
			AutoColor:  true,
		}

		// Create output prompt
		o.Create(server)

		psCon := &sConnect{
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
	s := &shell{
		Config:       config,
		Signal:       make(chan os.Signal),
		Run:          r,
		ServerList:   r.ServerList,
		Connects:     cons,
		PROMPT:       config.Prompt,
		History:      map[int]map[string]*shellHistory{},
		HistoryMu:    new(sync.Mutex),
		HistoryFile:  config.HistoryFile,
		currentConns: cons,
		Options: shellOption{
			LocalCommandNotRecordResult: false,
		},
	}

	// set signal
	// TODO: Windows対応
	//   - 参考: https://cad-san.hatenablog.com/entry/2017/01/09/170213
	signal.Notify(s.Signal, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)

	// old history list
	var historyCommand []string
	oldHistory, err := s.GetHistoryFromFile()
	if err == nil {
		for _, h := range oldHistory {
			historyCommand = append(historyCommand, h.Command)
		}
	}

	// check keepalive
	go func() {
		for {
			s.checkKeepalive()
			time.Sleep(3 * time.Second)
		}
	}()

	// create complete data
	// TODO(blacknon): 定期的に裏で取得するよう処理を加える(v0.6.1)
	s.GetCommandComplete()

	// create go-prompt
	p := prompt.New(
		s.Executor,
		s.Completer,
		prompt.OptionHistory(historyCommand),
		prompt.OptionLivePrefix(s.CreatePrompt),
		prompt.OptionInputTextColor(prompt.Green),
		prompt.OptionPrefixTextColor(prompt.Blue),
		prompt.OptionCompletionWordSeparator(" /\\,:\""),
		// Keybind
		// Alt+Backspace
		prompt.OptionAddASCIICodeBind(prompt.ASCIICodeBind{
			ASCIICode: []byte{0x1b, 0x7f},
			Fn:        prompt.DeleteWord,
		}),
		// Opt+LeftArrow
		prompt.OptionAddASCIICodeBind(prompt.ASCIICodeBind{
			ASCIICode: []byte{0x1b, 0x62},
			Fn:        prompt.GoLeftWord,
		}),
		// Opt+RightArrow
		prompt.OptionAddASCIICodeBind(prompt.ASCIICodeBind{
			ASCIICode: []byte{0x1b, 0x66},
			Fn:        prompt.GoRightWord,
		}),
		// Alt+LeftArrow
		prompt.OptionAddASCIICodeBind(prompt.ASCIICodeBind{
			ASCIICode: []byte{0x1b, 0x1b, 0x5B, 0x44},
			Fn:        prompt.GoLeftWord,
		}),
		// Alt+RightArrow
		prompt.OptionAddASCIICodeBind(prompt.ASCIICodeBind{
			ASCIICode: []byte{0x1b, 0x1b, 0x5B, 0x43},
			Fn:        prompt.GoRightWord,
		}),
		prompt.OptionSetExitCheckerOnInput(s.exitChecker),
	)

	// start go-prompt
	p.Run()

	return
}

// CreatePrompt is create shell prompt.
// default value is `[${COUNT}] <<< `
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

func (s *shell) exitChecker(in string, breakline bool) bool {
	if breakline {
		s.checkKeepalive()
	}

	if len(s.Connects) == 0 {
		s.exit(1, "Error: No valid connections\n")

		return true
	}

	return false
}

func (s *shell) exit(exitCode int, message string) {
	if message != "" {
		// error messages
		fmt.Print(message)
	}

	execLocalCommand(s.Config.PostCmd)
	os.Exit(exitCode)
}

// runCmdLocal exec command local machine.
func execLocalCommand(cmd string) {
	out, _ := exec.Command("sh", "-c", cmd).CombinedOutput()
	fmt.Print(string(out))
}
