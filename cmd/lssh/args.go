package main

import (
	"fmt"
	"os"
	"os/user"
	"sort"

	"github.com/blacknon/lssh/check"
	"github.com/blacknon/lssh/conf"
	"github.com/blacknon/lssh/list"
	sshcmd "github.com/blacknon/lssh/ssh"
	"github.com/urfave/cli"
)

func Lssh() (app *cli.App) {
	// Default config file path
	usr, _ := user.Current()
	defConf := usr.HomeDir + "/.lssh.conf"

	// Set help templete
	cli.AppHelpTemplate = `NAME:
    {{.Name}} - {{.Usage}}
USAGE:
    {{.HelpName}} {{if .VisibleFlags}}[options]{{end}} [commands...]
    {{if len .Authors}}
AUTHOR:
    {{range .Authors}}{{ . }}{{end}}
    {{end}}{{if .Commands}}
COMMANDS:
    {{range .Commands}}{{if not .HideHelp}}{{join .Names ", "}}{{ "\t"}}{{.Usage}}{{ "\n" }}{{end}}{{end}}{{end}}{{if .VisibleFlags}}
OPTIONS:
    {{range .VisibleFlags}}{{.}}
    {{end}}{{end}}{{if .Copyright }}
COPYRIGHT:
    {{.Copyright}}
    {{end}}{{if .Version}}
VERSION:
    {{.Version}}
    {{end}}
USAGE:
    # connect ssh
    {{.Name}}

    # parallel run command in select server over ssh
    {{.Name}} -p command...

    # parallel run command in select server over ssh, do it interactively.
    {{.Name}} -s
`

	// Create app
	app = cli.NewApp()
	app.Name = "lssh"
	app.Usage = "TUI list select and parallel ssh client command."
	app.Copyright = "blacknon(blacknon@orebibou.com)"
	app.Version = "0.6.0"

	// TODO(blacknon): オプションの追加
	//     -f      ... バックグラウンドでの接続(X11接続をバックグラウンドで実行する場合など)
	//     -X      ... X11の有効(configより優先)
	//     -r,--rc ... ローカルのbashrcを利用して接続するかどうかの指定(configより優先)
	//     -w      ... コマンド実行時にサーバ名ヘッダの表示をする
	//     -W      ... コマンド実行時にサーバ名ヘッダの表示をしない

	// Set options
	app.Flags = []cli.Flag{
		cli.StringSliceFlag{Name: "host,H", Usage: "connect servernames"},
		cli.StringFlag{Name: "file,f", Value: defConf, Usage: "config file path"},
		cli.StringFlag{Name: "portforward-local", Usage: "local port forwarding parameter (local port(ex. 127.0.0.1:8080))"},
		cli.StringFlag{Name: "portforward-remote", Usage: "local port forwarding parameter (remote port(ex. 127.0.0.1:80))"},
		cli.BoolFlag{Name: "list,l", Usage: "print server list from config"},
		cli.BoolFlag{Name: "term,t", Usage: "run specified command at terminal"},
		cli.BoolFlag{Name: "pshell,s", Usage: "use parallel-shell(pshell) (alpha)"},
		cli.BoolFlag{Name: "parallel,p", Usage: "run command parallel node(tail -F etc...)"},
		cli.BoolFlag{Name: "x11,X", Usage: "x11 forwarding(forward to ${DISPLAY})"},
		cli.BoolFlag{Name: "help,h", Usage: "print this help"},
	}
	app.EnableBashCompletion = true
	app.HideHelp = true

	// Run command action
	app.Action = func(c *cli.Context) error {
		// show help messages
		if c.Bool("help") {
			cli.ShowAppHelp(c)
			os.Exit(0)
		}

		hosts := c.StringSlice("host")
		confpath := c.String("file")

		// Get config data
		data := conf.ReadConf(confpath)

		// Set `exec command` or `shell` flag
		isMulti := false
		if len(c.Args()) > 0 || c.Bool("pshell") {
			isMulti = true
		}

		// Extraction server name list from 'data'
		names := conf.GetNameList(data)
		sort.Strings(names)

		// Check list flag
		if c.Bool("list") {
			fmt.Fprintf(os.Stdout, "lssh Server List:\n")
			for v := range names {
				fmt.Fprintf(os.Stdout, "  %s\n", names[v])
			}
			os.Exit(0)
		}

		selected := []string{}
		if len(hosts) > 0 {
			if !check.ExistServer(hosts, names) {
				fmt.Fprintln(os.Stderr, "Input Server not found from list.")
				os.Exit(1)
			} else {
				selected = hosts
			}
		} else {
			// View List And Get Select Line
			l := new(list.ListInfo)
			l.Prompt = "lssh>>"
			l.NameList = names
			l.DataList = data
			l.MultiFlag = isMulti

			l.View()
			selected = l.SelectName
			if selected[0] == "ServerName" {
				fmt.Fprintln(os.Stderr, "Server not selected.")
				os.Exit(1)
			}
		}

		r := new(sshcmd.Run)
		r.ServerList = selected
		r.Conf = data
		switch {
		case c.Bool("pshell") == true:
			r.Mode = "pshell"
		case len(c.Args()) > 0:
			r.Mode = "cmd"
		default:
			r.Mode = "shell"
		}

		r.ExecCmd = c.Args()
		r.X11 = c.Bool("x11")
		r.IsParallel = c.Bool("parallel")
		r.IsTerm = c.Bool("term")

		r.PortForwardLocal = c.String("portforward-local")
		r.PortForwardRemote = c.String("portforward-remote")

		r.Start()
		return nil
	}
	return app
}
