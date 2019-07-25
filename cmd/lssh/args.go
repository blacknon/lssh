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
	//     -f       ... バックグラウンドでの接続(X11接続やport forwardingをバックグラウンドで実行する場合など)。
	//                  「ssh -f」と同じ。
	//                  (https://github.com/sevlyar/go-daemon)
	//     -D       ... 何も実行しない(-fと組み合わせて利用するためのオプション)
	//     -a       ... 自動接続モード(接続が切れてしまった場合、自動的に再接続を試みる)。autossh的なoptionとして追加。
	//     -A <num> ... 自動接続モード(接続が切れてしまった場合、自動的に再接続を試みる)。再試行の回数指定。
	//     -w       ... コマンド実行時にサーバ名ヘッダの表示をする
	//     -W       ... コマンド実行時にサーバ名ヘッダの表示をしない

	// Set options
	app.Flags = []cli.Flag{
		// common option
		cli.StringSliceFlag{Name: "host,H", Usage: "connect `servername`."},
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config `filepath`."},

		// port forward option
		cli.BoolFlag{Name: "local-forward,L", Usage: "Local port forward mode."},
		cli.BoolFlag{Name: "remote-forward,R", Usage: "Remote port forward mode."},
		cli.StringFlag{Name: "dynamic-forward,D", Usage: "Dynamic port forward mode(Socks5). Specify a `port`."},
		cli.StringFlag{Name: "portforward-local", Usage: "port forwarding parameter, `address:port`. use local-forward or reverse-forward. (local port(ex. 127.0.0.1:8080))."},
		cli.StringFlag{Name: "portforward-remote", Usage: "port forwarding parameter, `address:port`. use local-forward or reverse-forward. (remote port(ex. 127.0.0.1:80))."},

		// Other bool
		cli.BoolFlag{Name: "background,f", Usage: "run background."}, // TODO(blacknon): create
		cli.BoolFlag{Name: "x11,X", Usage: "x11 forwarding(forward to ${DISPLAY})."},
		cli.BoolFlag{Name: "term,t", Usage: "run specified command at terminal."},
		cli.BoolFlag{Name: "parallel,p", Usage: "run command parallel node(tail -F etc...)."},
		cli.BoolFlag{Name: "localrc", Usage: "use local bashrc shell."},
		cli.BoolFlag{Name: "not-localrc", Usage: "not use local bashrc shell."},
		cli.BoolFlag{Name: "pshell,s", Usage: "use parallel-shell(pshell) (alpha)."},
		cli.BoolFlag{Name: "list,l", Usage: "print server list from config."},
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

		// exec command
		r.ExecCmd = c.Args()
		r.IsParallel = c.Bool("parallel")

		// x11 forwarding
		r.X11 = c.Bool("x11")

		// is tty
		r.IsTerm = c.Bool("term")

		// local bashrc use
		r.IsBashrc = c.Bool("localrc")
		r.IsNotBashrc = c.Bool("not-localrc")

		// local/remote port forwarding mode
		switch {
		case c.Bool("local-forward"):
			r.PortForwardMode = "L"
		case c.Bool("remote-forward"):
			r.PortForwardMode = "R"
		case c.Bool("local-forward") && c.Bool("remote-forward"):
			r.PortForwardMode = "R"
		default:
			r.PortForwardMode = ""
		}

		// local/remote port forwarding address
		r.PortForwardLocal = c.String("portforward-local")
		r.PortForwardRemote = c.String("portforward-remote")

		// Dynamic port forwarding port
		r.DynamicPortForward = c.String("dynamic-forward")

		r.Start()
		return nil
	}
	return app
}
