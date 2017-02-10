package main

import (
	"fmt"
	"os"
	"os/user"
	"sort"
	"strings"

	arg "github.com/alexflint/go-arg"
	"github.com/blacknon/lssh/check"
	"github.com/blacknon/lssh/conf"
	"github.com/blacknon/lssh/list"
	"github.com/blacknon/lssh/ssh"
)

type CommandOption struct {
	FilePath string `arg:"-f,help:config file path"`
	Exec     string `arg:"-e,help:exec_command"`
}

func main() {
	// Exec Before Check
	check.OsCheck()
	check.CommandExistCheck()

	// Set default value
	defaultConf := "~/.lssh.conf"
	usr, _ := user.Current()
	defaultConfPath := strings.Replace(defaultConf, "~", usr.HomeDir, 1)

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

	// Get List
	listConf := conf.ConfigCheckRead(configFile)

	// Get Server Name List (and sort List)
	nameList := conf.GetNameList(listConf)
	sort.Strings(nameList)

	// View List And Get Select Line
	selectServer := list.DrawList(nameList, listConf)

	if selectServer == "ServerName" {
		fmt.Println("Server not selected.")
		os.Exit(0)
	}

	// Connect SSH
	ssh.ConnectSsh(selectServer, listConf, execRemoteCmd)
}
