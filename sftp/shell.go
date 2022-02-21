// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
	"github.com/kballard/go-shellquote"
)

// TODO(blacknon): 補完処理が遅い・不安定になってるので対処する
// TODO(blacknon): 補完処理で、複数ホスト接続時に一部ホストにしか存在しないディレクトリを指定した場合、そこで処理が止まってしまう挙動を修正する

// sftp Shell mode function
func (r *RunSftp) shell() {
	// start message
	fmt.Println("Start lsftp...")

	// print select server
	r.Run.PrintSelectServer()

	// create go-prompt
	p := prompt.New(
		r.Executor,
		r.Completer,
		prompt.OptionLivePrefix(r.CreatePrompt),
		prompt.OptionInputTextColor(prompt.Green),
		prompt.OptionPrefixTextColor(prompt.Blue),
		prompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator), // test
	)

	// start go-prompt
	p.Run()

	return
}

// Executor is sftp Shell mode function.
func (r *RunSftp) Executor(command string) {
	// trim space
	command = strings.TrimSpace(command)
	if len(command) == 0 {
		return
	}
	// re-escape
	reescape := regexp.MustCompile(`(\\)`)
	command = reescape.ReplaceAllString(command, `\\$1`)

	cmdline, _ := shellquote.Split(command)

	// switch command
	switch cmdline[0] {
	case "bye", "exit", "quit":
		os.Exit(0)
	case "help", "?":

	case "cat":
		r.cat(cmdline)
	case "cd": // change remote directory
		r.cd(cmdline)
	case "chgrp":
		r.chgrp(cmdline)
	case "chmod":
		r.chmod(cmdline)
	case "chown":
		r.chown(cmdline)
	// case "copy":
	case "df":
		r.df(cmdline)
	case "get":
		r.get(cmdline)
	case "lcat":
		r.lcat(cmdline)
	case "lcd":
		r.lcd(cmdline)
	case "lls":
		r.lls(cmdline)
	case "lmkdir":
		r.lmkdir(cmdline)
	// case "ln":
	case "lpwd":
		r.lpwd(cmdline)
	case "ls":
		r.ls(cmdline)
	// case "lumask":
	case "mkdir":
		r.mkdir(cmdline)
	case "put":
		r.put(cmdline)
	case "pwd":
		r.pwd(cmdline)
	case "rename":
		r.rename(cmdline)
	case "rm":
		r.rm(cmdline)
	case "rmdir":
		r.rmdir(cmdline)
	case "symlink":
		r.symlink(cmdline)
	// case "tree":
	// case "!": // ! or !command...
	case "": // none command...
	default:
		fmt.Println("Command Not Found...")
	}
}

// Completer is sftp Shell mode function
// TODO(blacknon): PATH補完については、flagを見て対象のコマンドラインの初回だけ行わせるようにする(プロンプトが切り替わる度にflagをfalse or trueにすることで対処？)
func (r *RunSftp) Completer(t prompt.Document) []prompt.Suggest {
	// result
	var suggest []prompt.Suggest

	// Get cursor left
	left := t.CurrentLineBeforeCursor()

	// Get cursor char(string)
	char := ""
	if len(left) > 0 {
		char = string(left[len(left)-1])
	}

	cmdline := strings.Split(left, " ")
	if len(cmdline) == 1 {
		suggest = []prompt.Suggest{
			{Text: "bye", Description: "Quit lsftp"},
			{Text: "cat", Description: "Open file"},
			{Text: "cd", Description: "Change remote directory to 'path'"},
			// {Text: "chgrp", Description: "Change group of file 'path' to 'grp'"},
			// {Text: "chown", Description: "Change owner of file 'path' to 'own'"},
			// {Text: "copy", Description: "Copy to file from 'remote' or 'local' to 'remote' or 'local'"},
			{Text: "df", Description: "Display statistics for current directory or filesystem containing 'path'"},
			{Text: "exit", Description: "Quit lsftp"},
			{Text: "get", Description: "Download file"},
			{Text: "help", Description: "Display this help text"},
			{Text: "lcat", Description: "Open local file"},
			{Text: "lcd", Description: "Change local directory to 'path'"},
			{Text: "lls", Description: "Display local directory listing"},
			{Text: "lmkdir", Description: "Create local directory"},
			// {Text: "ln", Description: "Link remote file (-s for symlink)"},
			{Text: "lpwd", Description: "Print local working directory"},
			{Text: "ls", Description: "Display remote directory listing"},
			// {Text: "lumask", Description: "Set local umask to 'umask'"},
			{Text: "mkdir", Description: "Create remote directory"},
			{Text: "put", Description: "Upload file"},
			{Text: "pwd", Description: "Display remote working directory"},
			{Text: "quit", Description: "Quit sftp"},
			{Text: "rename", Description: "Rename remote file"},
			{Text: "rm", Description: "Delete remote file"},
			{Text: "rmdir", Description: "Remove remote directory"},
			{Text: "symlink", Description: "Create symbolic link"},
			// {Text: "tree", Description: "Tree view remote directory"},
			// {Text: "!command", Description: "Execute 'command' in local shell"},
			{Text: "!", Description: "Escape to local shell"},
			{Text: "?", Description: "Display this help text"},
		}
	} else { // command pattern
		switch cmdline[0] {
		case "cd":
			return r.PathComplete(true, 1, t)
		case "cat":
			// TODO(blacknon): ファイル容量が大きいと途中で止まるっぽい。
			return r.PathComplete(true, 1, t)
		case "chgrp":
			// TODO(blacknon): そのうち追加 ver0.6.3
		case "chown":
			// TODO(blacknon): そのうち追加 ver0.6.3
		case "df":
			suggest = []prompt.Suggest{
				{Text: "-h", Description: "print sizes in powers of 1024 (e.g., 1023M)"},
				{Text: "-i", Description: "list inode information instead of block usage"},
			}
			return prompt.FilterHasPrefix(suggest, t.GetWordBeforeCursor(), false)
		case "get":
			// TODO(blacknon): オプションを追加したら引数の数から減らす処理が必要
			switch {
			case strings.Count(t.CurrentLineBeforeCursor(), " ") == 1: // remote
				return r.PathComplete(true, 1, t)
			case strings.Count(t.CurrentLineBeforeCursor(), " ") == 2: // local
				return r.PathComplete(false, 2, t)
			}
		case "lcat":
			return r.PathComplete(false, 1, t)
		case "lcd":
			return r.PathComplete(false, 1, t)
		case "lls":
			// switch options or path
			switch {
			case contains([]string{"-"}, char):
				suggest = []prompt.Suggest{
					{Text: "-1", Description: "list one file per line"},
					{Text: "-a", Description: "do not ignore entries starting with"},
					{Text: "-f", Description: "do not sort"},
					{Text: "-h", Description: "with -l, print sizes like 1K 234M 2G etc."},
					{Text: "-l", Description: "use a long listing format"},
					{Text: "-n", Description: "list numeric user and group IDs"},
					{Text: "-r", Description: "reverse order while sorting"},
					{Text: "-S", Description: "sort by file size, largest first"},
					{Text: "-t", Description: "sort by modification time, newest first"},
				}
				return prompt.FilterHasPrefix(suggest, t.GetWordBeforeCursor(), false)

			default:
				return r.PathComplete(false, 1, t)
			}
		case "lmkdir":
			switch {
			case contains([]string{"-"}, char):
				suggest = []prompt.Suggest{
					{Text: "-p", Description: "no error if existing, make parent directories as needed"},
				}
				return prompt.FilterHasPrefix(suggest, t.GetWordBeforeCursor(), false)

			default:
				return r.PathComplete(false, 1, t)
			}

		// case "ln":
		case "lpwd":
		case "ls":
			// switch options or path
			switch {
			case contains([]string{"-"}, char):
				suggest = []prompt.Suggest{
					{Text: "-1", Description: "list one file per line"},
					{Text: "-a", Description: "do not ignore entries starting with"},
					{Text: "-f", Description: "do not sort"},
					{Text: "-h", Description: "with -l, print sizes like 1K 234M 2G etc."},
					{Text: "-l", Description: "use a long listing format"},
					{Text: "-n", Description: "list numeric user and group IDs"},
					{Text: "-r", Description: "reverse order while sorting"},
					{Text: "-S", Description: "sort by file size, largest first"},
					{Text: "-t", Description: "sort by modification time, newest first"},
				}
				return prompt.FilterHasPrefix(suggest, t.GetWordBeforeCursor(), false)

			default:
				return r.PathComplete(true, 1, t)
			}

		// case "lumask":
		case "mkdir":
			switch {
			case contains([]string{"-"}, char):
				suggest = []prompt.Suggest{
					{Text: "-p", Description: "no error if existing, make parent directories as needed"},
				}

			default:
				return r.PathComplete(true, 1, t)
			}

		case "put":
			// TODO(blacknon): オプションを追加したら引数の数から減らす処理が必要
			switch {
			case strings.Count(t.CurrentLineBeforeCursor(), " ") == 1: // local
				return r.PathComplete(false, 1, t)
			case strings.Count(t.CurrentLineBeforeCursor(), " ") == 2: // remote
				return r.PathComplete(true, 2, t)
			}
		case "pwd":
		case "quit":
		case "rename":
			return r.PathComplete(true, 1, t)
		case "rm":
			return r.PathComplete(true, 1, t)
		case "rmdir":
			return r.PathComplete(true, 1, t)
		case "symlink":
			// TODO(blacknon): そのうち追加 ver0.6.2
		// case "tree":

		default:
		}
	}

	// return prompt.FilterHasPrefix(suggest, t.GetWordBeforeCursor(), true)
	return prompt.FilterHasPrefix(suggest, t.GetWordBeforeCursor(), false)
}

// PathComplete return path complete data
func (r *RunSftp) PathComplete(remote bool, num int, t prompt.Document) []prompt.Suggest {
	// suggest
	var suggest []prompt.Suggest

	// Get cursor left
	left := t.CurrentLineBeforeCursor()

	// Get cursor char(string)
	char := ""
	if len(left) > 0 {
		char = string(left[len(left)-1])
	}

	// get last slash place
	word := t.GetWordBeforeCursor()
	sp := strings.LastIndex(word, "/")
	if len(word) > 0 {
		word = word[sp+1:]
	}

	switch remote {
	case true:
		// update r.RemoteComplete
		switch {
		case contains([]string{"/"}, char): // char is slach or
			r.GetRemoteComplete(t.GetWordBeforeCursor())
		case contains([]string{" "}, char) && strings.Count(t.CurrentLineBeforeCursor(), " ") == num:
			r.GetRemoteComplete(t.GetWordBeforeCursor())
		}
		suggest = r.RemoteComplete

	case false:
		// update r.RemoteComplete
		switch {
		case contains([]string{"/"}, char): // char is slach or
			r.GetLocalComplete(t.GetWordBeforeCursor())
		case contains([]string{" "}, char) && strings.Count(t.CurrentLineBeforeCursor(), " ") == num:
			r.GetLocalComplete(t.GetWordBeforeCursor())
		}
		suggest = r.LocalComplete

	}

	return prompt.FilterHasPrefix(suggest, word, false)
}

// GetRemoteComplete set r.RemoteComplete
func (r *RunSftp) GetRemoteComplete(path string) {
	// create map
	m := map[string][]string{}
	exit := make(chan bool)

	// create suggest slice
	var p []prompt.Suggest

	// create sync mutex
	sm := new(sync.Mutex)

	// connect client...
	for s, c := range r.Client {
		server := s
		client := c

		go func() {
			// set rpath
			var rpath string
			switch {
			case filepath.IsAbs(path):
				rpath = path
			case !filepath.IsAbs(path):
				rpath = filepath.Join(client.Pwd, path)
			}

			// check rpath
			stat, err := client.Connect.Stat(rpath)
			if err != nil {
				exit <- true
				return
			}

			if stat.IsDir() {
				rpath = rpath + "/*"
			} else {
				rpath = rpath + "*"
			}

			// get path list
			globlist, err := client.Connect.Glob(rpath)
			if err != nil {
				exit <- true
				return
			}

			// set glob list
			for _, p := range globlist {
				p = filepath.Base(p)

				// escape blob
				re := regexp.MustCompile(`([][ )(\\])`)
				p = re.ReplaceAllString(p, "\\$1")

				sm.Lock()
				m[p] = append(m[p], server)
				sm.Unlock()
			}
			exit <- true
		}()
	}

	// wait
	for i := 0; i < len(r.Client); i++ {
		<-exit
	}

	// create suggest
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

	// sort
	sort.SliceStable(p, func(i, j int) bool { return p[i].Text < p[j].Text })

	// set suggest to struct
	r.RemoteComplete = p
}

// GetLocalComplete set r.LocalComplete
func (r *RunSftp) GetLocalComplete(path string) {
	// create suggest slice
	var p []prompt.Suggest
	stat, err := os.Lstat(path)
	if err != nil {
		return
	}

	// dir check
	var lpath string
	if stat.IsDir() {
		lpath = path + "/*"
	} else {
		lpath = path + "*"
	}

	// get globlist
	globlist, err := filepath.Glob(lpath)
	if err != nil {
		return
	}

	// set path
	for _, lp := range globlist {
		lp = filepath.Base(lp)
		suggest := prompt.Suggest{
			Text:        lp,
			Description: "local path.",
		}

		p = append(p, suggest)
	}

	r.LocalComplete = p
}

// CreatePrompt return prompt string.
func (r *RunSftp) CreatePrompt() (p string, result bool) {
	p = "lsftp>> "
	return p, true
}

func contains(s []string, e string) bool {
	for _, v := range s {
		if e == v {
			return true
		}
	}
	return false
}
