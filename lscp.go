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

	selectServer := []string{}
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

		selectServer = l.SelectName
		if selectServer[0] == "ServerName" {
			fmt.Fprintln(os.Stderr, "Server not selected.")
			os.Exit(1)
		}

		if fromHostType == "local" {
			toSelectServer = selectServer
		} else {
			fromSelectServer = selectServer
		}
	}

	// Check file exisits
	if fromHostType == "local" {
		_, err := os.Stat(conf.GetFullPath(fromPath))
		if err != nil {
			fmt.Fprintf(os.Stderr, "not found path %s \n", fromPath)
			os.Exit(1)
		}
		fromPath = conf.GetFullPath(fromPath)
	}
	if toHostType == "local" {
		_, err := os.Stat(conf.GetFullPath(toPath))
		if err != nil {
			fmt.Fprintf(os.Stderr, "not found path %s \n", toPath)
			os.Exit(1)
		}
		toPath = conf.GetFullPath(toPath)
	}

	r_scp := new(scp.RunInfoScp)
	r_scp.CopyFromType = fromHostType
	r_scp.CopyFromPath = fromPath
	r_scp.CopyFromServer = fromSelectServer
	r_scp.CopyToType = toHostType
	r_scp.CopyToPath = toPath
	r_scp.CopyToServer = toSelectServer
	r_scp.ConConfig = listConf

	// print from
	if r_scp.CopyFromType == "local" {
		fmt.Fprintf(os.Stderr, "From %s:%s\n", r_scp.CopyFromType, r_scp.CopyFromPath)
	} else {
		fmt.Println(fromSelectServer)
		fmt.Fprintf(os.Stderr, "From %s(%s):%s\n", r_scp.CopyFromType, strings.Join(r_scp.CopyFromServer, ","), r_scp.CopyFromPath)
	}

	// print to
	if r_scp.CopyToType == "local" {
		fmt.Fprintf(os.Stderr, "To   %s:%s\n", r_scp.CopyToType, r_scp.CopyToPath)
	} else {
		fmt.Fprintf(os.Stderr, "To   %s(%s):%s\n", r_scp.CopyToType, strings.Join(r_scp.CopyToServer, ","), r_scp.CopyToPath)
	}

	r_scp.ScpRun()
}
