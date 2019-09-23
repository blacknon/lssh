// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"fmt"
	"os"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
)

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
		// prompt.OptionPrefix(pShellPrompt),
		prompt.OptionLivePrefix(r.CreatePrompt),
		prompt.OptionInputTextColor(prompt.Green),
		prompt.OptionPrefixTextColor(prompt.Blue),
		prompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator), // test
	)

	// start go-prompt
	p.Run()

	return
}

// sftp Shell mode function
func (r *RunSftp) Executor(command string) {
	// trim space
	command = strings.TrimSpace(command)

	// parse command
	cmdline := strings.Split(command, " ")

	// switch command
	switch cmdline[0] {
	case "bye", "exit", "quit":
		os.Exit(0)
	case "help", "?":

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
	case "lcd":
		r.lcd(cmdline)
	case "lls":
	case "lmkdir":
	// case "ln":
	case "lpwd":
		r.lpwd(cmdline)
	case "ls":
		r.ls(cmdline)
	case "lumask":
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
			// {Text: "copy", Description: "Copy to file from 'remote' or 'local' to 'remote' or 'local'"},
			{Text: "df", Description: "Display statistics for current directory or filesystem containing 'path'"},
			{Text: "exit", Description: "Quit lsftp"},
			{Text: "get", Description: "Download file"},
			// {Text: "reget", Description: "Resume download file"},
			// {Text: "reput", Description: "Resume upload file"},
			{Text: "help", Description: "Display this help text"},
			{Text: "lcd", Description: "Change local directory to 'path'"},
			{Text: "lls", Description: "Display local directory listing"},
			{Text: "lmkdir", Description: "Create local directory"},
			// {Text: "ln", Description: "Link remote file (-s for symlink)"},
			{Text: "lpwd", Description: "Print local working directory"},
			{Text: "ls", Description: "Display remote directory listing"},
			{Text: "lumask", Description: "Set local umask to 'umask'"},
			{Text: "mkdir", Description: "Create remote directory"},
			// {Text: "progress", Description: "Toggle display of progress meter"},
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
		case "chgrp":
		case "chown":
		case "df":
			suggest = []prompt.Suggest{
				{Text: "-h", Description: "print sizes in powers of 1024 (e.g., 1023M)"},
				{Text: "-i", Description: "list inode information instead of block usage"},
			}
		case "get":
		case "lcd":
		case "lls":
		case "lmkdir":
		// case "ln":
		case "lpwd":
		case "ls":
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
		case "lumask":
		case "mkdir":
			suggest = []prompt.Suggest{
				{Text: "-p", Description: "no error if existing, make parent directories as needed"},
			}
		case "put":
		case "pwd":
		case "quit":
		case "rename":
		case "rm":
		case "rmdir":
		case "symlink":
			// case "tree":
		}
	}

	return prompt.FilterHasPrefix(suggest, t.GetWordBeforeCursor(), true)
}

//
func (r *RunSftp) GetCompleteRemotePath() {

}

//
func (r *RunSftp) GetCompleteLocalPath() {

}

func (r *RunSftp) CreatePrompt() (p string, result bool) {
	p = "lsftp>> "
	return p, true
}
