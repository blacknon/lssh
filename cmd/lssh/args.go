// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"

	"github.com/blacknon/lssh/check"
	"github.com/blacknon/lssh/common"
	"github.com/blacknon/lssh/conf"
	"github.com/blacknon/lssh/list"
	sshcmd "github.com/blacknon/lssh/ssh"
	"github.com/urfave/cli"
)

func Lssh() (app *cli.App) {
	// Default config file path
	defConf := common.GetDefaultConfigPath()

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

    # run command selected server over ssh.
    {{.Name}} command...

    # run command parallel in selected server over ssh.
    {{.Name}} -p command...
`

	// Create app
	app = cli.NewApp()
	// app.UseShortOptionHandling = true
	app.Name = "lssh"
	app.Usage = "TUI list select and parallel ssh client command."
	app.Copyright = "blacknon(blacknon@orebibou.com)"
	app.Version = "0.6.12"

	// TODO(blacknon): オプションの追加
	//     -T       ... マウント・リバースマウントのTypeを指定できるようにする(v0.7.0)
	//                  ※ そもそもfuseをそのままfusemountでマウントできるのか？という謎もある
	//     -f       ... バックグラウンドでの接続(X11接続やport forwardingをバックグラウンドで実行する場合など)。
	//                  「ssh -f」と同じ。 (v0.7.0)
	//                  (https://github.com/sevlyar/go-daemon)
	//     -a       ... 自動接続モード(接続が切れてしまった場合、自動的に再接続を試みる)。autossh的なoptionとして追加。  (v0.7.0)
	//     --autoconnect <num>
	//              ... 自動接続モード(接続が切れてしまった場合、自動的に再接続を試みる)。再試行の回数指定(デフォルトは3回?)。  (v0.7.0)
	//     --read_profile
	//              ... デフォルトではlocalrc読み込みでのshellではsshサーバ上のprofileは読み込まないが、このオプションを指定することで読み込まれるようになる (v0.7.0)
	//     -P
	//              ... 3muxを用いたマルチプレクサでのParallel Shell/Command実行を有効にする(v0.7.0)

	// Set options
	app.Flags = []cli.Flag{
		// common option
		cli.StringSliceFlag{Name: "host,H", Usage: "connect `servername`."},
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config `filepath`."},

		// port forward (with dynamic forward) option
		cli.StringSliceFlag{Name: "L", Usage: "Local port forward mode.Specify a `[bind_address:]port:remote_address:port`. Only single connection works."},
		cli.StringSliceFlag{Name: "R", Usage: "Remote port forward mode.Specify a `[bind_address:]port:remote_address:port`. If only one port is specified, it will operate as Reverse Dynamic Forward. Only single connection works."},
		cli.StringFlag{Name: "D", Usage: "Dynamic port forward mode(Socks5). Specify a `port`. Only single connection works."},
		cli.StringFlag{Name: "d", Usage: "HTTP Dynamic port forward mode. Specify a `port`. Only single connection works."},
		cli.StringFlag{Name: "r", Usage: "HTTP Reverse Dynamic port forward mode. Specify a `port`. Only single connection works."},
		cli.StringFlag{Name: "M", Usage: "NFS Dynamic forward mode. Specify a `port:/path/to/remote`. Only single connection works."},
		cli.StringFlag{Name: "m", Usage: "NFS Reverse Dynamic forward mode. Specify a `port:/path/to/local`. Only single connection works."},

		// Other bool
		cli.BoolFlag{Name: "w", Usage: "Displays the server header when in command execution mode."},
		cli.BoolFlag{Name: "W", Usage: "Not displays the server header when in command execution mode."},
		cli.BoolFlag{Name: "not-execute,N", Usage: "not execute remote command and shell."},
		cli.BoolFlag{Name: "X11,X", Usage: "Enable x11 forwarding(forward to ${DISPLAY})."},
		cli.BoolFlag{Name: "Y", Usage: "Enable trusted x11 forwarding(forward to ${DISPLAY})."},
		cli.BoolFlag{Name: "term,t", Usage: "run specified command at terminal."},
		cli.BoolFlag{Name: "parallel,p", Usage: "run command parallel node(tail -F etc...)."},
		cli.BoolFlag{Name: "localrc", Usage: "use local bashrc shell."},
		cli.BoolFlag{Name: "not-localrc", Usage: "not use local bashrc shell."},
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
		data := conf.Read(confpath)

		// Set `exec command` or `shell` flag
		isMulti := false
		if (len(c.Args()) > 0 || c.Bool("pshell")) && !c.Bool("not-execute") {
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

			// Check selected
			if len(selected) == 0 {
				fmt.Fprintln(os.Stderr, "Server config is not set.")
				os.Exit(1)
			}
			if selected[0] == "ServerName" {
				fmt.Fprintln(os.Stderr, "Server not selected.")
				os.Exit(1)
			}
		}

		r := new(sshcmd.Run)
		r.ServerList = selected
		r.Conf = data
		switch {
		case c.Bool("pshell") == true && !c.Bool("not-execute"):
			r.Mode = "pshell"
		case len(c.Args()) > 0 && !c.Bool("not-execute"):
			// Becomes a shell when not-execute is given.
			r.Mode = "cmd"
		default:
			r.Mode = "shell"
		}

		// exec command
		r.ExecCmd = c.Args()
		r.IsParallel = c.Bool("parallel")

		// x11 forwarding
		enableX11 := c.Bool("X11")
		enableTrustedX11 := c.Bool("Y")

		if enableX11 || enableTrustedX11 {
			r.X11 = true
		}
		if enableTrustedX11 {
			r.X11Trusted = true
		}

		// is tty
		r.IsTerm = c.Bool("term")

		// local bashrc use
		r.IsBashrc = c.Bool("localrc")
		r.IsNotBashrc = c.Bool("not-localrc")

		// set w/W flag
		if c.Bool("w") {
			fmt.Println("enable w")
			r.EnableHeader = true
		}
		if c.Bool("W") {
			fmt.Println("enable W")
			r.DisableHeader = true
		}

		// Set port forwards
		var err error
		var forwards []*conf.PortForward

		// Set local port forwarding
		for _, forwardargs := range c.StringSlice("L") {
			f := new(conf.PortForward)
			f.Mode = "L"
			f.Local, f.Remote, err = common.ParseForwardPort(forwardargs)
			forwards = append(forwards, f)
		}

		// Set remote port forwarding
		for _, forwardargs := range c.StringSlice("R") {
			f := new(conf.PortForward)
			f.Mode = "R"

			// If only numbers are passed as arguments, treat as Reverse Dynamic Port Forward
			if regexp.MustCompile(`^[0-9]+$`).Match([]byte(forwardargs)) {
				r.ReverseDynamicPortForward = forwardargs
			} else {
				f.Local, f.Remote, err = common.ParseForwardPort(forwardargs)
				forwards = append(forwards, f)
			}
		}

		// Set NFS Forwarding
		nfsForwarding := c.String("n")
		if nfsForwarding != "" {
			port, path, err := common.ParseNFSForwardPortPath(nfsForwarding)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}

			r.NFSDynamicForwardPort = port
			r.NFSDynamicForwardPath = path
		}

		// Set NFS Reverse Forwarding
		nfsReverseForwarding := c.String("m")
		if nfsReverseForwarding != "" {
			port, path, err := common.ParseNFSForwardPortPath(nfsReverseForwarding)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}

			path = common.GetFullPath(path)

			r.NFSReverseDynamicForwardPort = port
			r.NFSReverseDynamicForwardPath = path
		}

		// if err
		if err != nil {
			fmt.Printf("Error: %s \n", err)
		}

		// is not execute
		r.IsNone = c.Bool("not-execute")

		// Local/Remote port forwarding port
		r.PortForward = forwards

		// Dynamic port forwarding port
		r.DynamicPortForward = c.String("D")

		// HTTP Dynamic port forwarding port
		r.HTTPDynamicPortForward = c.String("d")

		// HTTP Reverse Dynamic port forwarding port
		r.HTTPReverseDynamicPortForward = c.String("r")

		r.Start()
		return nil
	}
	return app
}
