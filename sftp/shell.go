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

	"github.com/blacknon/lssh/common"
	"github.com/c-bata/go-prompt"
	"github.com/kballard/go-shellquote"
)

// TODO(blacknon): 補完処理が遅い・不安定になってるので対処する

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
		// prompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator), // test
		prompt.OptionCompletionWordSeparator(" /\\,:"),
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
			switch {
			case strings.Count(t.CurrentLineBeforeCursor(), " ") == 1:
				return r.PathComplete(true, false, t)
			}
		case "cat":
			// TODO(blacknon): ファイル容量が大きいと途中で止まるっぽい。
			return r.PathComplete(true, false, t)
		case "chgrp":
			// TODO(blacknon): そのうち追加 ver0.6.3
		case "chown":
			// TODO(blacknon): そのうち追加 ver0.6.3
		case "df":
			// switch options or path
			switch {
			case contains([]string{"-"}, char):
				suggest = []prompt.Suggest{
					{Text: "-h", Description: "print sizes in powers of 1024 (e.g., 1023M)"},
					{Text: "-i", Description: "list inode information instead of block usage"},
				}
				return prompt.FilterHasPrefix(suggest, t.GetWordBeforeCursor(), false)

			default:
				return r.PathComplete(true, false, t)
			}

		case "get":
			// TODO(blacknon): オプションを追加したら引数の数から減らす処理が必要
			switch {
			case strings.Count(t.CurrentLineBeforeCursor(), " ") == 1: // remote
				return r.PathComplete(true, false, t)
			case strings.Count(t.CurrentLineBeforeCursor(), " ") >= 2: // remote and local
				return r.PathComplete(true, true, t)
			}
		case "lcat":
			return r.PathComplete(false, true, t)
		case "lcd":
			return r.PathComplete(false, true, t)
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
				return r.PathComplete(false, true, t)
			}
		case "lmkdir":
			switch {
			case contains([]string{"-"}, char):
				suggest = []prompt.Suggest{
					{Text: "-p", Description: "no error if existing, make parent directories as needed"},
				}
				return prompt.FilterHasPrefix(suggest, t.GetWordBeforeCursor(), false)

			default:
				return r.PathComplete(false, true, t)
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
				return r.PathComplete(true, false, t)
			}

		// case "lumask":
		case "mkdir":
			switch {
			case contains([]string{"-"}, char):
				suggest = []prompt.Suggest{
					{Text: "-p", Description: "no error if existing, make parent directories as needed"},
				}

			default:
				return r.PathComplete(true, false, t)
			}

		case "put":
			// TODO(blacknon): オプションを追加したら引数の数から減らす処理が必要
			switch {
			case strings.Count(t.CurrentLineBeforeCursor(), " ") == 1: // local
				return r.PathComplete(false, true, t)
			case strings.Count(t.CurrentLineBeforeCursor(), " ") >= 2: // local and remote
				return r.PathComplete(true, true, t)
			}
		case "pwd":
		case "quit":
		case "rename":
			return r.PathComplete(true, false, t)
		case "rm":
			return r.PathComplete(true, false, t)
		case "rmdir":
			return r.PathComplete(true, false, t)
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
func (r *RunSftp) PathComplete(remote, local bool, t prompt.Document) []prompt.Suggest {
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

	confirmRemote := false
	if remote {
		if strings.Count(t.CurrentLineBeforeCursor(), ":") == 0 && strings.Count(t.CurrentLineBeforeCursor(), "/") == 0 && strings.Count(t.CurrentLineBeforeCursor(), ",") >= 1 {
			wordlist := strings.Split(word, ",")
			word = wordlist[len(wordlist)-1]
		}

		// update r.RemoteComplete
		switch {
		// host set
		// case contains([]string{","}, char) && strings.Count(t.CurrentLineBeforeCursor(), " ") == 0:
		case contains([]string{","}, char):
			confirmRemote = r.GetRemoteComplete(true, false, t.GetWordBeforeCursor())

		case contains([]string{":"}, char):
			confirmRemote = r.GetRemoteComplete(false, true, t.GetWordBeforeCursor())

		// char is slach or
		case contains([]string{"/"}, char):
			confirmRemote = r.GetRemoteComplete(false, true, t.GetWordBeforeCursor())

		case contains([]string{" "}, char):
			confirmRemote = r.GetRemoteComplete(true, true, t.GetWordBeforeCursor())

		}
		suggest = append(suggest, r.RemoteComplete...)
	}

	if local && !confirmRemote {
		// update r.RemoteComplete
		switch {
		case contains([]string{"/"}, char) || contains([]string{" "}, char): // char is slach
			r.GetLocalComplete(t.GetWordBeforeCursor())
		}
		suggest = append(suggest, r.LocalComplete...)
	}

	// return prompt.FilterHasPrefix(suggest, word, false)
	return prompt.FilterHasPrefix(suggest, word, true)
}

// GetRemoteComplete set r.RemoteComplete
func (r *RunSftp) GetRemoteComplete(ishost, ispath bool, path string) (confirmRemote bool) {
	// confirm remote
	confirmRemote = false

	// create map
	m := map[string][]string{}
	exit := make(chan bool)

	// create suggest slice
	var p []prompt.Suggest
	var s []prompt.Suggest

	// create sync mutex
	sm := new(sync.Mutex)

	// target maps
	targetmap := map[string]*SftpConnect{}

	// get r.Client keys
	servers := make([]string, 0, len(r.Client))
	for k := range r.Client {
		servers = append(servers, k)
	}

	// create suggest (hosts)
	for _, server := range servers {
		// create suggest
		suggest := prompt.Suggest{
			Text:        server,
			Description: "remote host.",
		}

		// append ps.Complete
		s = append(s, suggest)
	}

	// If it is confirmed that it is a completion of the host name
	// create suggest(hostname)
	if ishost && !ispath {
		confirmRemote = true
		r.RemoteComplete = s
		return
	}

	// parse path
	parsedservers, parsedPath := common.ParseHostPath(path)
	if len(parsedservers) == 0 {
		targetmap = r.Client
	}

	for server, client := range r.Client {
		if common.Contains(parsedservers, server) {
			targetmap[server] = client
		}
	}

	// connect client...
	for s, c := range targetmap {
		server := s
		client := c

		go func() {
			// set rpath
			var rpath string
			switch {
			case filepath.IsAbs(parsedPath):
				rpath = parsedPath
			case !filepath.IsAbs(parsedPath):
				rpath = filepath.Join(client.Pwd, parsedPath)
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
	for i := 0; i < len(targetmap); i++ {
		<-exit
	}

	// create suggest(path)
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
	if ispath && !ishost {
		r.RemoteComplete = p
	} else {
		r.RemoteComplete = p
		r.RemoteComplete = append(r.RemoteComplete, s...)
	}

	return
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
