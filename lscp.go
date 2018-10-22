package main

import (
	"os"

	"github.com/blacknon/lssh/args"
)

// Command Option
// type CommandOption struct {
// 	Host       []string `arg:"-H,help:connect servername"`
// 	File       string   `arg:"-f,help:config file path"`
// 	Permission bool     `arg:"-p,help:copy file permission"`
// 	From       []string `arg:"positional,required,help:copy from path (local:<path>|remote:<path>)"`
// 	To         string   `arg:"positional,required,help:copy to path (local:<path>|remote:<path>)"`
// }

// // Version Setting
// func (CommandOption) Version() string {
// 	return "lscp v0.5.1"
// }

func main() {
	app := args.Lscp()
	app.Run(os.Args)
	// // get Command Option
	// var args struct {
	// 	CommandOption
	// }
	// arg.MustParse(&args)

	// // set option value
	// configFile := args.File
	// if configFile == "" {
	// 	usr, _ := user.Current()
	// 	defaultConfPath := usr.HomeDir + "/.lssh.conf"
	// 	configFile = defaultConfPath
	// }
	// connectHost := args.Host
	// isPermission := args.Permission

	// // Check scp type remote
	// isFromInRemote := false
	// isFromInLocal := false
	// for _, from := range args.From {
	// 	// parse args
	// 	isFromRemote, _ := check.ParseScpPath(from)

	// 	if isFromRemote {
	// 		isFromInRemote = true
	// 	} else {
	// 		isFromInLocal = true
	// 	}
	// }
	// isToRemote, _ := check.ParseScpPath(args.To)

	// // Check from and to Type
	// check.CheckTypeError(isFromInRemote, isFromInLocal, isToRemote, len(connectHost))

	// // Get config data
	// data := conf.ReadConf(configFile)

	// // Get Server Name List (and sort List)
	// nameList := conf.GetNameList(data)
	// sort.Strings(nameList)

	// selectServer := []string{}
	// toServer := []string{}
	// fromServer := []string{}

	// // view server list
	// switch {
	// // connectHost is set
	// case len(connectHost) != 0:
	// 	if check.ExistServer(connectHost, nameList) == false {
	// 		fmt.Fprintln(os.Stderr, "Input Server not found from list.")
	// 		os.Exit(1)
	// 	} else {
	// 		toServer = connectHost
	// 	}

	// // remote to remote scp
	// case isFromInRemote && isToRemote:
	// 	// View From list
	// 	from_l := new(list.ListInfo)
	// 	from_l.Prompt = "lscp(from)>>"
	// 	from_l.NameList = nameList
	// 	from_l.DataList = data
	// 	from_l.MultiFlag = false
	// 	from_l.View()
	// 	fromServer = from_l.SelectName
	// 	if fromServer[0] == "ServerName" {
	// 		fmt.Fprintln(os.Stderr, "Server not selected.")
	// 		os.Exit(1)
	// 	}

	// 	// View to list
	// 	to_l := new(list.ListInfo)
	// 	to_l.Prompt = "lscp(to)>>"
	// 	to_l.NameList = nameList
	// 	to_l.DataList = data
	// 	to_l.MultiFlag = true
	// 	to_l.View()
	// 	toServer = to_l.SelectName
	// 	if toServer[0] == "ServerName" {
	// 		fmt.Fprintln(os.Stderr, "Server not selected.")
	// 		os.Exit(1)
	// 	}

	// default:
	// 	// View List And Get Select Line
	// 	l := new(list.ListInfo)
	// 	l.Prompt = "lscp>>"
	// 	l.NameList = nameList
	// 	l.DataList = data
	// 	l.MultiFlag = true
	// 	l.View()

	// 	selectServer = l.SelectName
	// 	if selectServer[0] == "ServerName" {
	// 		fmt.Fprintln(os.Stderr, "Server not selected.")
	// 		os.Exit(1)
	// 	}

	// 	if isFromInRemote {
	// 		fromServer = selectServer
	// 	} else {
	// 		toServer = selectServer
	// 	}
	// }

	// // scp struct
	// runScp := new(ssh.RunScp)

	// // set from info
	// for _, from := range args.From {
	// 	// parse args
	// 	isFromRemote, fromPath := check.ParseScpPath(from)

	// 	// Check local file exisits
	// 	if !isFromRemote {
	// 		_, err := os.Stat(common.GetFullPath(fromPath))
	// 		if err != nil {
	// 			fmt.Fprintf(os.Stderr, "not found path %s \n", fromPath)
	// 			os.Exit(1)
	// 		}
	// 		fromPath = common.GetFullPath(fromPath)
	// 	}

	// 	// set from data
	// 	runScp.From.IsRemote = isFromRemote
	// 	runScp.From.Path = append(runScp.From.Path, fromPath)
	// }
	// runScp.From.Server = fromServer

	// // set to info
	// isToRemote, toPath := check.ParseScpPath(args.To)
	// runScp.To.IsRemote = isToRemote
	// runScp.To.Path = []string{toPath}
	// runScp.To.Server = toServer

	// runScp.Permission = isPermission
	// runScp.Config = data

	// // // print from
	// // if !runScp.From.IsRemote {
	// // 	fmt.Fprintf(os.Stderr, "From %s:%s\n", runScp.From.Type, runScp.From.Path)
	// // } else {
	// // 	fmt.Fprintf(os.Stderr, "From %s(%s):%s\n", runScp.From.Type, strings.Join(runScp.From.Server, ","), runScp.From.Path)
	// // }

	// // // print to
	// // if !runScp.To.IsRemote {
	// // 	fmt.Fprintf(os.Stderr, "To   %s:%s\n", runScp.To.Type, runScp.To.Path)
	// // } else {
	// // 	fmt.Fprintf(os.Stderr, "To   %s(%s):%s\n", runScp.To.Type, strings.Join(runScp.To.Server, ","), runScp.To.Path)
	// // }

	// runScp.Start()
}
