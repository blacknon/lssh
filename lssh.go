package main

import (
	"fmt"
	"os"
	"os/user"
	"sort"
	"strings"

	arg "github.com/alexflint/go-arg"
	"github.com/blacknon/lssh/conf"
	"github.com/blacknon/lssh/list"
	"github.com/blacknon/lssh/ssh"
)

// Command Option
type CommandOption struct {
	Host     []string `arg:"-H,help:Connect servername"`
	List     bool     `arg:"-l,help:print server list"`
	File     string   `arg:"-f,help:config file path"`
	Terminal bool     `arg:"-t,help:Run specified command at terminal"`
	Parallel bool     `arg:"-p,help:Exec command parallel node(tail -F etc...)"`
	Generate bool     `arg:"help:(beta) generate .lssh.conf from .ssh/config.(not support ProxyCommand)"`
	Command  []string `arg:"-c,help:Remote Server exec command."`
}

// Version Setting
func (CommandOption) Version() string {
	return "lssh v0.4.5"
}

func main() {
	// Exec Before Check
	conf.CheckBeforeStart()

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
	listFlag := args.List
	terminalExec := args.Terminal
	parallelExec := args.Parallel
	generateFlag := args.Generate
	execRemoteCmd := args.Command

	// Generate .lssh.conf
	if generateFlag == true {
		conf.GenerateConfig()
		os.Exit(0)
	}

	// Get config data
	listConf := conf.ReadConf(configFile)

	// Set exec command flag
	cmdFlag := false
	if len(execRemoteCmd) != 0 {
		cmdFlag = true
	}

	// Extraction server name list from 'listConf'
	nameList := conf.GetNameList(listConf)
	sort.Strings(nameList)

	// check list flag
	if listFlag == true {
		fmt.Fprintf(os.Stdout, "lssh Server List:\n")
		for v := range nameList {
			fmt.Fprintf(os.Stdout, "  %s\n", nameList[v])
		}
		os.Exit(0)
	}

	selectServer := []string{}
	if len(connectHost) != 0 {
		if conf.CheckInputServerExit(connectHost, nameList) == false {
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
		l.MultiFlag = cmdFlag

		l.View()
		selectServer = l.SelectName
		if selectServer[0] == "ServerName" {
			fmt.Fprintln(os.Stderr, "Server not selected.")
			os.Exit(1)
		}
	}

	// Exec Connect ssh
	if cmdFlag == true {
		// Print selected server and connect command
		fmt.Fprintf(os.Stderr, "Select Server :%s\n", strings.Join(selectServer, ","))
		fmt.Fprintf(os.Stderr, "Exec command  :%s\n", strings.Join(execRemoteCmd, " "))

		// Connect SSH
		c := new(ssh.RunInfoCmd)
		c.ServerList = selectServer
		c.ConfList = listConf
		c.Tflag = terminalExec
		c.Pflag = parallelExec
		c.ExecCmd = execRemoteCmd
		c.ConSshCmd()

		os.Exit(0)
	} else {
		// Print selected server
		fmt.Fprintf(os.Stderr, "Select Server :%s\n", selectServer[0])

		// No select Server
		if len(selectServer) > 1 {
			fmt.Fprintln(os.Stderr, "Connect ssh interactive shell. Connect only to the first device")
		}

		// Connect SSH Terminal
		c := new(ssh.ConInfoTerm)
		c.Log, c.LogDir = listConf.Log.Enable, listConf.Log.Dir
		c.Server = selectServer[0]
		c.User = listConf.Server[c.Server].User
		c.Addr = listConf.Server[c.Server].Addr
		c.Port = listConf.Server[c.Server].Port
		c.Pass = listConf.Server[c.Server].Pass
		c.KeyPath = listConf.Server[c.Server].Key
		c.BeforeCmd = listConf.Server[c.Server].BeforeCmd
		c.AfterCmd = listConf.Server[c.Server].AfterCmd
		c.ProxyServer = listConf.Server[c.Server].ProxyServer
		c.Proxy = listConf.Server[c.Server].Proxy

		err := c.Connect()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}
