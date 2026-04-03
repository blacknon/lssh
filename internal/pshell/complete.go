// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package pshell

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	libpath "path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/c-bata/go-prompt"
	"golang.org/x/crypto/ssh"
)

func safeCreateSession(c *sConnect) (session *ssh.Session, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			session = nil
			err = fmt.Errorf("create session panic: %v", rec)
		}
	}()

	if c == nil || c.Connect == nil || c.Connect.Client == nil {
		return nil, fmt.Errorf("invalid connect")
	}

	return c.CreateSession()
}

// TODO(blacknon): `!!`や"`:$`についても実装を行う
// TODO(blacknon): `!command`だとまとめてパイプ経由でデータを渡すことになっているが、`!!command`で個別のローカルコマンドにデータを渡すように実装する

// Completer parallel-shell complete function
func (s *shell) Completer(t prompt.Document) []prompt.Suggest {
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
				{Text: "%outexec", Description: "%outexec <-n num> command..., exec local command with output result. result is in env variable."},
				// outの出力でdiffをするためのローカルコマンド。すべての出力と比較するのはあまりに辛いと思われるため、最初の出力との比較、といった方式で対応するのが良いか？？
				// {Text: "%diff", Description: "%diff [num], show history result list."},
			}
			c = append(c, buildin...)

			// get remote and local command complete data
			c = append(c, s.CmdComplete...)

			// return
			return prompt.FilterHasPrefix(c, t.GetWordBeforeCursor(), false)

		case checkBuildInCommand(c): // if build-in command.
			var suggest []prompt.Suggest
			switch c {
			// %out
			case "%out":
				for i := 0; i < len(s.History); i++ {
					var cmd string
					for _, h := range s.History[i] {
						cmd = h.Command
					}

					s := prompt.Suggest{
						Text:        strconv.Itoa(i),
						Description: cmd,
					}
					suggest = append(suggest, s)
				}

			// %outexec
			case "%outexec":
				// switch options or path
				switch {
				case contains([]string{"-"}, char):
					suggest = []prompt.Suggest{
						{Text: "--help", Description: "help message"},
						{Text: "-h", Description: "help message"},
						{Text: "-n", Description: "set history number"},
					}

				case "-n " == t.GetWordBeforeCursorWithSpace():
					for i := 0; i < len(s.History); i++ {
						var cmd string
						for _, h := range s.History[i] {
							cmd = h.Command
						}

						s := prompt.Suggest{
							Text:        strconv.Itoa(i),
							Description: cmd,
						}
						suggest = append(suggest, s)
					}

				default:
					suggest = s.GetLocalhostCommandComplete()
				}

			}

			return prompt.FilterHasPrefix(suggest, t.GetWordBeforeCursor(), false)

		default:
			switch {
			case contains([]string{"/"}, char): // char is slach or
				s.PathComplete = s.GetPathComplete(!checkLocalCommand(c), t.GetWordBeforeCursor())
			case contains([]string{" "}, char) && strings.Count(t.CurrentLineBeforeCursor(), " ") == 1:
				s.PathComplete = s.GetPathComplete(!checkLocalCommand(c), t.GetWordBeforeCursor())
			}

			// get last slash place
			word := t.GetWordBeforeCursor()
			sp := strings.LastIndex(word, "/")
			if len(word) > 0 {
				word = word[sp+1:]
			}

			return prompt.FilterHasPrefix(s.PathComplete, word, false)
		}
	}

	return prompt.FilterHasPrefix(nil, t.GetWordBeforeCursor(), false)
}

// GetLocalhostCommandComplete
func (s *shell) GetLocalhostCommandComplete() (suggest []prompt.Suggest) {
	// bash complete command. use `compgen`.
	compCmd := []string{"compgen", "-c"}
	command := strings.Join(compCmd, " ")

	// get local machine command complete
	local, _ := exec.Command("bash", "-c", command).Output()
	rd := strings.NewReader(string(local))
	sc := bufio.NewScanner(rd)
	for sc.Scan() {
		s := prompt.Suggest{
			Text:        sc.Text(),
			Description: "Command. from:localhost",
		}
		suggest = append(suggest, s)
	}

	return suggest
}

// GetCommandComplete get command list remote machine.
// mode ... command/path
// data ... Value being entered
func (s *shell) GetCommandComplete() {
	// bash complete command. use `compgen`.
	compCmd := []string{"compgen", "-c"}
	command := strings.Join(compCmd, " ")

	// get local machine command complete
	local, _ := exec.Command("bash", "-c", command).Output()
	rd := strings.NewReader(string(local))
	sc := bufio.NewScanner(rd)
	for sc.Scan() {
		suggest := prompt.Suggest{
			Text:        "+" + sc.Text(),
			Description: "Command. from:localhost",
		}
		s.CmdComplete = append(s.CmdComplete, suggest)
	}

	// get remote machine command complete
	// command map
	cmdMap := map[string][]string{}

	// append command to cmdMap
	for _, c := range s.Connects {
		if c == nil || c.Connect == nil || c.Connect.Client == nil {
			continue
		}

		// Create buffer
		buf := new(bytes.Buffer)

		// Create session, and output to buffer
		session, err := safeCreateSession(c)
		if err != nil || session == nil {
			continue
		}
		session.Stdout = buf

		// Run get complete command
		_ = session.Run(command)
		_ = session.Close()

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

		// append s.Complete
		s.CmdComplete = append(s.CmdComplete, suggest)
	}

	sort.SliceStable(s.CmdComplete, func(i, j int) bool { return s.CmdComplete[i].Text < s.CmdComplete[j].Text })
}

// GetPathComplete return complete path from local or remote machine.
// TODO(blacknon): 複数のノードにあるPATHだけ補完リストに出てる状態なので、単一ノードにしか無いファイルも出力されるよう修正する
func (s *shell) GetPathComplete(remote bool, word string) (p []prompt.Suggest) {
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
		for _, c := range s.Connects {
			con := c
			go func() {
				if con == nil || con.Connect == nil || con.Connect.Client == nil {
					exit <- true
					return
				}

				// Create buffer
				buf := new(bytes.Buffer)

				// Create session, and output to buffer
				session, err := safeCreateSession(con)
				if err != nil || session == nil {
					exit <- true
					return
				}
				session.Stdout = buf

				// Run get complete command
				_ = session.Run(command)
				_ = session.Close()

				// Scan and put completed command to map.
				sc := bufio.NewScanner(buf)
				for sc.Scan() {
					sm.Lock()

					path := libpath.Base(sc.Text())

					m[path] = append(m[path], con.Name)
					sm.Unlock()
				}

				exit <- true
			}()
		}

		for i := 0; i < len(s.Connects); i++ {
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

			// append s.Complete
			p = append(p, suggest)
		}

	case !remote: // is local machine
		if runtime.GOOS != "windows" {
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
