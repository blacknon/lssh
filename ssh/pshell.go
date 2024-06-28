// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/conf"
	"github.com/blacknon/lssh/output"
	"github.com/c-bata/go-prompt"
)

// TODO(blacknon): 接続が切れた場合の再接続処理、および再接続ができなかった場合のsliceからの削除対応の追加(v0.7.0)
// TODO(blacknon): pShellのログ(実行コマンド及び出力結果)をログとしてファイルに記録する機能の追加(v0.7.0) => 任意のファイルを指定するように
// TODO(blacknon): グループ化(`()`で囲んだりする)や三項演算子への対応(v0.7.0)
// TODO(blacknon): `サーバ名:command...` で、指定したサーバでのみコマンドを実行させる機能の追加(v0.6.8)
// TODO(blacknon): petをうまいこと利用できるような仕組みを作る(v0.7.0)
// TODO(blacknon): parallel shellでkeybindや関数が使えるような仕組みを作る(どうやってやるかは不明だが…)(v0.7.0)

// TODO(blacknon):
//     出力をvim diffに食わせてdiffを得られるようにしたい => 変数かプロセス置換か、なにかしらの方法でローカルコマンド実行時にssh経由で得られた出力を食わせる方法を実装する？
//     => 多分、プロセス置換が良いんだと思う(プロセス置換時にssh先でコマンドを実行できるように、かつ実行したデータを個別にファイルとして扱えるようにしたい)
//        ```bash
//        !vimdiff <(cat /etc/passwd)
//        => !vimdiff host1:/etc/passwd host2:/etc/passwd ....
//        ```
//     やるなら普通に一時ファイルに書き出すのが良さそう(/tmp 配下とか。一応、ちゃんと権限周り気をつけないといかんね、というのと消さないといかんね、というお気持ち)

// Pshell is Parallel-Shell struct
type pShell struct {
	Config        conf.ShellConfig
	Signal        chan os.Signal
	Count         int
	ServerList    []string
	Connects      []*psConnect
	PROMPT        string
	History       map[int]map[string]*pShellHistory
	HistoryFile   string
	latestCommand string
	CmdComplete   []prompt.Suggest
	PathComplete  []prompt.Suggest
	Options       pShellOption
}

// pShellOption is optitons pshell.
// TODO(blacknon): つくる。
type pShellOption struct {
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

// psConnect is pShell connect struct.
type psConnect struct {
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

func (r *Run) pshell() (err error) {
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
	var cons []*psConnect
	for _, server := range r.ServerList {
		// Create *sshlib.Connect
		con, err := r.CreateSshConnect(server)
		if err != nil {
			log.Println(err)
			continue
		}

		// TTY enable
		con.TTY = true

		// Create Output
		o := &output.Output{
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
		Config:      config,
		Signal:      make(chan os.Signal),
		ServerList:  r.ServerList,
		Connects:    cons,
		PROMPT:      config.Prompt,
		History:     map[int]map[string]*pShellHistory{},
		HistoryFile: config.HistoryFile,
		Options: pShellOption{
			LocalCommandNotRecordResult: true, // debug
		},
	}

	// set signal
	// TODO: Windows対応
	//   - 参考: https://cad-san.hatenablog.com/entry/2017/01/09/170213
	signal.Notify(ps.Signal, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)

	// old history list
	var historyCommand []string
	oldHistory, err := ps.GetHistoryFromFile()
	if err == nil {
		for _, h := range oldHistory {
			historyCommand = append(historyCommand, h.Command)
		}
	}

	// check keepalive
	go func() {
		for {
			ps.checkKeepalive()
			time.Sleep(3 * time.Second)
		}
	}()

	// create complete data
	// TODO(blacknon): 定期的に裏で取得するよう処理を加える(v0.6.1)
	ps.GetCommandComplete()

	// create go-prompt
	p := prompt.New(
		ps.Executor,
		ps.Completer,
		prompt.OptionHistory(historyCommand),
		prompt.OptionLivePrefix(ps.CreatePrompt),
		prompt.OptionInputTextColor(prompt.Green),
		prompt.OptionPrefixTextColor(prompt.Blue),
		prompt.OptionCompletionWordSeparator("/: \\"), // test
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
		prompt.OptionSetExitCheckerOnInput(ps.exitChecker),
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

func (ps *pShell) exitChecker(in string, breakline bool) bool {
	if breakline {
		ps.checkKeepalive()
	}

	if len(ps.Connects) == 0 {
		// error messages
		fmt.Printf("Error: No valid connections\n")

		// TODO: 再接続が発生する場合はexitせずに返す？
		ps.exit(1)

		return true
	}

	return false
}

func (ps *pShell) exit(exitCode int) {
	execLocalCommand(ps.Config.PostCmd)
	os.Exit(exitCode)
}
