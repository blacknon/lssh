// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"bufio"
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/c-bata/go-prompt"
)

// TODO(blacknon): `!!`や`!$`についても実装を行う
// TODO(blacknon): `!command`だとまとめてパイプ経由でデータを渡すことになっているが、`!!command`で個別のローカルコマンドにデータを渡すように実装する

// Completer parallel-shell complete function
func (ps *pShell) Completer(t prompt.Document) []prompt.Suggest {
	// if currente line data is none.
	if len(t.CurrentLine()) == 0 {
		return prompt.FilterHasPrefix(nil, t.GetWordBeforeCursor(), false)
	}

	// Get cursor left
	left := t.CurrentLineBeforeCursor()
	pslice, err := parsePipeLine(left)
	if err != nil {
		return prompt.FilterHasPrefix(nil, t.GetWordBeforeCursor(), false)
	}

	// Get cursor char(string)
	char := ""
	if len(left) > 0 {
		char = string(left[len(left)-1])
	}

	sl := len(pslice) // pline slice count
	ll := 0
	num := 0
	if sl >= 1 {
		ll = len(pslice[sl-1])             // pline count
		num = len(pslice[sl-1][ll-1].Args) // pline args count
	}

	if sl >= 1 && ll >= 1 {
		c := pslice[sl-1][ll-1].Args[0]

		// switch suggest
		switch {
		case num <= 1 && !contains([]string{" ", "|"}, char): // if command
			var c []prompt.Suggest

			// build-in command suggest
			buildin := []prompt.Suggest{
				{Text: "exit", Description: "exit lssh shell"},
				{Text: "quit", Description: "exit lssh shell"},
				{Text: "clear", Description: "clear screen"},
				{Text: "%history", Description: "show history"},
				{Text: "%out", Description: "%out [num], show history result."},
				{Text: "%outlist", Description: "%outlist, show history result list."},
				// outの出力でdiffをするためのローカルコマンド。すべての出力と比較するのはあまりに辛いと思われるため、最初の出力との比較、といった方式で対応するのが良いか？？
				// {Text: "%diff", Description: "%diff [num], show history result list."},
			}
			c = append(c, buildin...)

			// get remote and local command complete data
			c = append(c, ps.CmdComplete...)

			// return
			return prompt.FilterHasPrefix(c, t.GetWordBeforeCursor(), false)

		case checkBuildInCommand(c): // if build-in command.
			var a []prompt.Suggest
			switch c {
			case "%out":
				for i := 0; i < len(ps.History); i++ {
					var cmd string
					for _, h := range ps.History[i] {
						cmd = h.Command
					}

					suggest := prompt.Suggest{
						Text:        strconv.Itoa(i),
						Description: cmd,
					}
					a = append(a, suggest)
				}
			}

			return prompt.FilterHasPrefix(a, t.GetWordBeforeCursor(), false)

		default:
			switch {
			case contains([]string{"/"}, char): // char is slach or
				ps.PathComplete = ps.GetPathComplete(!checkLocalCommand(c), t.GetWordBeforeCursor())
			case contains([]string{" "}, char) && strings.Count(t.CurrentLineBeforeCursor(), " ") == 1:
				ps.PathComplete = ps.GetPathComplete(!checkLocalCommand(c), t.GetWordBeforeCursor())
			}

			// get last slash place
			word := t.GetWordBeforeCursor()
			sp := strings.LastIndex(word, "/")
			if len(word) > 0 {
				word = word[sp+1:]
			}

			return prompt.FilterHasPrefix(ps.PathComplete, word, false)
		}
	}

	return prompt.FilterHasPrefix(nil, t.GetWordBeforeCursor(), false)
}

// GetCommandComplete get command list remote machine.
// mode ... command/path
// data ... Value being entered
func (ps *pShell) GetCommandComplete() {
	// bash complete command. use `compgen`.
	compCmd := []string{"compgen", "-c"}
	command := strings.Join(compCmd, " ")

	// get local machine command complete
	local, _ := exec.Command("bash", "-c", command).Output()
	rd := strings.NewReader(string(local))
	sc := bufio.NewScanner(rd)
	for sc.Scan() {
		suggest := prompt.Suggest{
			Text:        "!" + sc.Text(),
			Description: "Command. from:localhost",
		}
		ps.CmdComplete = append(ps.CmdComplete, suggest)
	}

	// get remote machine command complete
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
		sort.Strings(hosts)
		h := strings.Join(hosts, ",")

		// create suggest
		suggest := prompt.Suggest{
			Text:        cmd,
			Description: "Command. from:" + h,
		}

		// append ps.Complete
		ps.CmdComplete = append(ps.CmdComplete, suggest)
	}

	sort.SliceStable(ps.CmdComplete, func(i, j int) bool { return ps.CmdComplete[i].Text < ps.CmdComplete[j].Text })
}

// GetPathComplete return complete path from local or remote machine.
// TODO(blacknon): 複数のノードにあるPATHだけ補完リストに出てる状態なので、単一ノードにしか無いファイルも出力されるよう修正する
func (ps *pShell) GetPathComplete(remote bool, word string) (p []prompt.Suggest) {
	compCmd := []string{"compgen", "-f", word}
	command := strings.Join(compCmd, " ")

	switch {
	case remote: // is remote machine
		// create map
		m := map[string][]string{}

		exit := make(chan bool)

		// create sync mutex
		sm := new(sync.Mutex)

		// append path to m
		for _, c := range ps.Connects {
			con := c
			go func() {
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
					sm.Lock()

					var path string
					if runtime.GOOS != "windows" {
						path = filepath.Base(sc.Text())
					} else {
						path = sc.Text()
					}
					m[path] = append(m[path], con.Name)
					sm.Unlock()
				}

				exit <- true
			}()
		}

		for i := 0; i < len(ps.Connects); i++ {
			<-exit
		}

		// m to suggest
		for path, hosts := range m {
			// join hosts
			h := strings.Join(hosts, ",")

			// create suggest
			suggest := prompt.Suggest{
				Text:        path,
				Description: "remote path. from:" + h,
			}

			// append ps.Complete
			p = append(p, suggest)
		}

	case !remote: // is local machine
		sgt, _ := exec.Command("bash", "-c", command).Output()
		rd := strings.NewReader(string(sgt))
		sc := bufio.NewScanner(rd)
		for sc.Scan() {
			suggest := prompt.Suggest{
				Text: filepath.Base(sc.Text()),
				// Text:        sc.Text(),
				Description: "local path.",
			}
			p = append(p, suggest)
		}
	}

	sort.SliceStable(p, func(i, j int) bool { return p[i].Text < p[j].Text })
	return
}

func contains(s []string, e string) bool {
	for _, v := range s {
		if e == v {
			return true
		}
	}
	return false
}
