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
	sshcmd "github.com/blacknon/lssh/ssh"
)

// Command Option
type CommandOption struct {
	Host     []string `arg:"-H,help:connect servername"`
	List     bool     `arg:"-l,help:print server list"`
	File     string   `arg:"-f,help:config file path"`
	Terminal bool     `arg:"-t,help:run specified command at terminal"`
	Parallel bool     `arg:"-p,help:run command parallel node(tail -F etc...)"`
	Generate bool     `arg:"help:(beta) generate .lssh.conf from .ssh/config.(not support ProxyCommand)"`
	Command  []string `arg:"-c,help:remote Server exec command."`
}

// Version Setting
func (CommandOption) Version() string {
	return "lssh v0.5.1"
}

func main() {
	// get Command Option
	var args struct {
		CommandOption
	}
	arg.MustParse(&args)

	// set option value
	configFile := args.File
	if configFile == "" {
		usr, _ := user.Current()
		defaultConfPath := usr.HomeDir + "/.lssh.conf"
		configFile = defaultConfPath
	}
	connectHost := args.Host
	isListView := args.List
	isTerminal := args.Terminal
	isParallel := args.Parallel
	isGenerate := args.Generate
	runCommand := args.Command

	// Generate .lssh.conf
	if isGenerate {
		conf.GenConf()
		os.Exit(0)
	}

	// Get config data
	listConf := conf.ReadConf(configFile)

	// Set exec command flag
	isMultiSelect := false
	if len(runCommand) > 0 {
		isMultiSelect = true
	}

	// Extraction server name list from 'listConf'
	nameList := conf.GetNameList(listConf)
	sort.Strings(nameList)

	// check list flag
	if isListView {
		fmt.Fprintf(os.Stdout, "lssh Server List:\n")
		for v := range nameList {
			fmt.Fprintf(os.Stdout, "  %s\n", nameList[v])
		}
		os.Exit(0)
	}

	selectServer := []string{}
	if len(connectHost) > 0 {
		if !check.ExistServer(connectHost, nameList) {
			fmt.Fprintln(os.Stderr, "Input Server not found from list.")
			os.Exit(1)
		} else {
			selectServer = connectHost
		}
	} else {
		// View List And Get Select Line
		l := new(list.ListInfo)
		l.Prompt = "lssh>>"
		l.NameList = nameList
		l.DataList = listConf
		l.MultiFlag = isMultiSelect

		l.View()
		selectServer = l.SelectName
		if selectServer[0] == "ServerName" {
			fmt.Fprintln(os.Stderr, "Server not selected.")
			os.Exit(1)
		}
	}

	r := new(sshcmd.Run)
	r.ServerList = selectServer
	r.Conf = listConf
	r.IsTerm = isTerminal
	r.IsParallel = isParallel
	r.ExecCmd = runCommand
	r.Start()
}
