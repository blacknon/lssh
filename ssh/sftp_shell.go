// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"fmt"
	"os"
	"strings"

	"github.com/c-bata/go-prompt"
)

// sftp Shell mode function
func (r *RunSftp) shell() {
	// start message
	fmt.Println("Start lsftp...")

	// print select server
	r.Run.printSelectServer()

	// create go-prompt
	p := prompt.New(
		r.Executor,
		r.Completer,
		// prompt.OptionPrefix(pShellPrompt),
		prompt.OptionLivePrefix(r.CreatePrompt),
		prompt.OptionInputTextColor(prompt.Green),
		prompt.OptionPrefixTextColor(prompt.Blue),
		// prompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator), // test
	)

	// start go-prompt
	p.Run()

	return
}

// sftp Shell mode function
func (r *RunSftp) Executor(command string) {
	cmdline := strings.Split(command, " ")
	switch cmdline[0] {
	case "bye", "exit", "quit":
		os.Exit(0)
	case "help", "?":
	case "cd": // change remote directory
		r.cd(cmdline[1])
	case "chgrp":
	case "chown":
	case "cp":
	case "df":
	case "reget":
	case "reput":
	case "lcd":
	case "lls":
	case "lmkdir":
	case "ln":
	case "lpwd":
	case "ls":
		r.ls(command)
	case "lumask":
	case "mkdir":
	case "progress":
	case "put":
	case "pwd":
	case "rename":
	case "rm":
	case "rmdir":
	case "symlink":
	case "tree":
	case "version":
	case "!": // ! or !command...
	}
}

// sftp Shell mode function
func (r *RunSftp) Completer(t prompt.Document) []prompt.Suggest {
	// result
	var suggest []prompt.Suggest

	// Get cursor left
	left := t.CurrentLineBeforeCursor()
	cmdline := strings.Split(left, " ")
	if len(cmdline) == 1 {
		suggest = []prompt.Suggest{
			{Text: "bye", Description: "Quit lsftp"},
			{Text: "cd", Description: "Change remote directory to 'path'"},
			{Text: "chgrp", Description: "Change group of file 'path' to 'grp'"},
			{Text: "chown", Description: "Change owner of file 'path' to 'own'"},
			{Text: "copy", Description: "Copy to file from 'remote' or 'local' to 'remote' or 'local'"},
			{Text: "df", Description: "Display statistics for current directory or filesystem containing 'path'"},
			{Text: "exit", Description: "Quit lsftp"},
			{Text: "get", Description: "Download file"},
			{Text: "reget", Description: "Resume download file"},
			{Text: "reput", Description: "Resume upload file"},
			{Text: "help", Description: "Display this help text"},
			{Text: "lcd", Description: "Change local directory to 'path'"},
			{Text: "lls", Description: "Display local directory listing"},
			{Text: "lmkdir", Description: "Create local directory"},
			{Text: "ln", Description: "Link remote file (-s for symlink)"},
			{Text: "lpwd", Description: "Print local working directory"},
			{Text: "ls", Description: "Display remote directory listing"},
			{Text: "lumask", Description: "Set local umask to 'umask'"},
			{Text: "mkdir", Description: "Create remote directory"},
			{Text: "progress", Description: "Toggle display of progress meter"},
			{Text: "put", Description: "Upload file"},
			{Text: "pwd", Description: "Display remote working directory"},
			{Text: "quit", Description: "Quit sftp"},
			{Text: "rename", Description: "Rename remote file"},
			{Text: "rm", Description: "Delete remote file"},
			{Text: "rmdir", Description: "Remove remote directory"},
			{Text: "symlink", Description: "Symlink remote file"},
			{Text: "tree", Description: "Tree view remote directory"},
			{Text: "version", Description: "Show SFTP version"},
			{Text: "!command", Description: "Execute 'command' in local shell"},
			{Text: "!", Description: "Escape to local shell"},
			{Text: "?", Description: "Display this help text"},
		}
	} else { // command pattern
		switch cmdline[0] {
		case "cd":
		case "chgrp":
		case "chown":
		case "df":
		case "get":
		case "reget":
		case "reput":
		case "lcd":
		case "lls":
		case "lmkdir":
		case "ln":
		case "lpwd":
		case "ls":
		case "lumask":
		case "mkdir":
		case "progress":
		case "put":
		case "pwd":
		case "quit":
		case "rename":
		case "rm":
		case "rmdir":
		case "symlink":
		case "tree":
		case "version":
		}
	}

	return prompt.FilterHasPrefix(suggest, t.GetWordBeforeCursor(), true)
}

func (r *RunSftp) CreatePrompt() (p string, result bool) {
	p = "lsftp>> "
	return p, true
}
