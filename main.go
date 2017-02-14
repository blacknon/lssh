package main

import (
	"fmt"
	"os"
	"os/user"
	"sort"

	arg "github.com/alexflint/go-arg"
	"github.com/blacknon/lssh/check"
	"github.com/blacknon/lssh/conf"
	"github.com/blacknon/lssh/list"
	"github.com/blacknon/lssh/ssh"
)

var (
	lsshVersion = "0.1"
)

type CommandOption struct {
	Version      bool   `arg:"-v,help:lssh version"`
	FilePath     string `arg:"-f,help:config file path"`
	Exec         string `arg:"-e,help:exec_command"`
	TerminalExec bool   `arg:"-T,help:Run specified command at terminal"`
}

func main() {
	// Exec Before Check
	check.OsCheck()
	check.DefCommandExistCheck()

	// Set default value
	usr, _ := user.Current()
	defaultConfPath := usr.HomeDir + "/.lssh.conf"

	// get Command Option
	var args struct {
		CommandOption
	}

	// Default Value
	args.FilePath = defaultConfPath
	args.Exec = ""
	arg.MustParse(&args)

	// set option value
	configFile := args.FilePath
	execRemoteCmd := args.Exec
	terminalExec := args.TerminalExec
	version := args.Version

	// View Version
	if version == true {
		fmt.Printf("lssh version %s \n", lsshVersion)
		os.Exit(0)
	}

	// Get List
	listConf := conf.ConfigCheckRead(configFile)

	// Get Server Name List (and sort List)
	nameList := conf.GetNameList(listConf)
	sort.Strings(nameList)

	// View List And Get Select Line
	selectServer := list.DrawList(nameList, listConf)

	if selectServer == "ServerName" {
		fmt.Println("Server not selected.")
		os.Exit(1)
	}

	if terminalExec == false && execRemoteCmd != "" {
		// Connect SSH Terminal
		os.Exit(ssh.ConnectSshCommand(selectServer, listConf, execRemoteCmd))
	} else {
		// Exec SSH Command Only
		os.Exit(ssh.ConnectSshTerminal(selectServer, listConf, execRemoteCmd))
	}
}
