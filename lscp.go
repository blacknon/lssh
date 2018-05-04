package main

import (
	"fmt"
	"os"
	"os/user"
	"sort"

	arg "github.com/alexflint/go-arg"
	"github.com/blacknon/lssh/conf"
	"github.com/blacknon/lssh/list"
	"github.com/blacknon/lssh/scp"
)

// Command Option
type CommandOption struct {
	Host        []string `arg:"-H,help:Connect servername"`
	File        string   `arg:"-f,help:config file path"`
	Recursively bool     `arg:"-r,help:Recursively copy entire directories."`
	From        string   `arg:"positional,required,help:Copy from path"`
	To          string   `arg:"positional,required,help:Copy to path"`
}

// Version Setting
func (CommandOption) Version() string {
	return "lscp v0.5.0"
}

func main() {
	// Exec Before Check
	conf.CheckBeforeStart()

	// Set default value
	usr, _ := user.Current()
	defaultConfPath := usr.HomeDir + "/.lssh.conf"

	// get Command Option
	var args struct {
		CommandOption
	}

	// Default Value
	args.File = defaultConfPath
	arg.MustParse(&args)

	// set option value
	connectHost := args.Host
	configFile := args.File
	//RecursivelyFlag := args.Recursively
	copyFrom := args.From
	copyTo := args.To

	// Check and Parse path args
	fromHostType, fromPath, fromResult := conf.ParsePathArg(copyFrom)
	toHostType, toPath, toResult := conf.ParsePathArg(copyTo)

	// Check fromResult,toResult
	if fromResult == false || toResult == false {
		fmt.Fprintln(os.Stderr, "The format of the specified argument is incorrect.")
		os.Exit(1)
	}

	// Check HostType local only
	if fromHostType == "local" && toHostType == "local" {
		fmt.Fprintln(os.Stderr, "It does not correspond local to local copy.")
		os.Exit(1)
	}

	// Check HostType remote only and Host flag
	if fromHostType == "remote" && toHostType == "remote" && len(connectHost) != 0 {
		fmt.Fprintln(os.Stderr, "In the case of remote to remote copy, it does not correspond to Host option.")
		os.Exit(1)
	}

	// Get List
	listConf := conf.ReadConf(configFile)

	// Get Server Name List (and sort List)
	nameList := conf.GetNameList(listConf)
	sort.Strings(nameList)

	toSelectServer := []string{}
	fromSelectServer := []string{}
	if len(connectHost) != 0 {
		if conf.CheckInputServerExit(connectHost, nameList) == false {
			fmt.Fprintln(os.Stderr, "Input Server not found from list.")
			os.Exit(1)
		} else {
			toSelectServer = connectHost
		}
	} else if fromHostType == "remote" && toHostType == "remote" {
		// View From list
		from_l := new(list.ListInfo)
		from_l.Prompt = "lscp(from)>>"
		from_l.NameList = nameList
		from_l.DataList = listConf
		from_l.MultiFlag = false
		from_l.View()
		fromSelectServer = from_l.SelectName
		if fromSelectServer[0] == "ServerName" {
			fmt.Fprintln(os.Stderr, "Server not selected.")
			os.Exit(1)
		}

		// View From list
		to_l := new(list.ListInfo)
		to_l.Prompt = "lscp(to)>>"
		to_l.NameList = nameList
		to_l.DataList = listConf
		to_l.MultiFlag = true
		to_l.View()
		toSelectServer = to_l.SelectName
		if toSelectServer[0] == "ServerName" {
			fmt.Fprintln(os.Stderr, "Server not selected.")
			os.Exit(1)
		}
	} else {
		// View List And Get Select Line
		l := new(list.ListInfo)
		l.Prompt = "lscp>>"
		l.NameList = nameList
		l.DataList = listConf
		l.MultiFlag = true
		l.View()

		toSelectServer = l.SelectName
		if toSelectServer[0] == "ServerName" {
			fmt.Fprintln(os.Stderr, "Server not selected.")
			os.Exit(1)
		}
	}

	r_scp := new(scp.RunInfoScp)
	r_scp.CopyFromType = fromHostType
	r_scp.CopyFromPath = fromPath
	r_scp.CopyToType = toHostType
	r_scp.CopyToPath = toPath
	r_scp.CopyFromServer = fromSelectServer
	r_scp.CopyToServer = toSelectServer
	r_scp.ConConfig = listConf

	r_scp.ScpRun()
}
