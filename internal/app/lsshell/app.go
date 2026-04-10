// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsshell

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"sort"

	"github.com/blacknon/lssh/internal/check"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/list"
	pshell "github.com/blacknon/lssh/internal/pshell"
	sshcmd "github.com/blacknon/lssh/internal/ssh"
	"github.com/blacknon/lssh/internal/version"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/urfave/cli"
)

func Lsshell() (app *cli.App) {
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
    # connect parallel ssh shell
	lsshell
`

	// Create app
	app = cli.NewApp()
	app.Name = "lsshell"
	app.Usage = "TUI list select and parallel ssh client shell."
	app.Copyright = "blacknon(blacknon@orebibou.com)"
	app.Version = version.AppVersion(app.Name)

	// Set options
	app.Flags = []cli.Flag{
		// common option
		cli.StringSliceFlag{Name: "host,H", Usage: "connect `servername`."},
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config `filepath`."},
		cli.StringFlag{Name: "generate-lssh-conf", Usage: "print generated lssh config from OpenSSH config to stdout (`~/.ssh/config` by default)."},

		// port forward option
		cli.StringSliceFlag{Name: "R", Usage: "Remote port forward mode.Specify a `[bind_address:]port:remote_address:port`. If only one port is specified, it will operate as Reverse Dynamic Forward. Only single connection works."},
		cli.StringFlag{Name: "r", Usage: "HTTP Reverse Dynamic port forward mode. Specify a `port`. Only single connection works."},
		cli.StringFlag{Name: "m", Usage: "NFS Reverse Dynamic forward mode. Specify a `port:/path/to/local`. Only single connection works."},

		// Other bool
		cli.BoolFlag{Name: "term,t", Usage: "run specified command at terminal."},
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

		if handled, err := conf.HandleGenerateConfigMode(c.String("generate-lssh-conf"), os.Stdout); handled {
			return err
		}

		// Get config data
		data, configErr := conf.ReadWithFallback(confpath, os.Stderr)
		if configErr != nil {
			return configErr
		}

		// Set `exec command` or `shell` flag
		isMulti := true

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

		// is tty
		r.IsTerm = c.Bool("term")

		// Set port forwards
		var err error
		var forwards []*conf.PortForward

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

		// if err
		if err != nil {
			fmt.Printf("Error: %s \n", err)
		}

		// Local/Remote port forwarding port
		r.PortForward = forwards
		r.HTTPReverseDynamicPortForward = c.String("r")
		if nfsReverseForwarding := c.String("m"); nfsReverseForwarding != "" {
			port, path, err := common.ParseNFSForwardPortPath(nfsReverseForwarding)
			if err != nil {
				return err
			}
			r.NFSReverseDynamicForwardPort = port
			r.NFSReverseDynamicForwardPath = common.GetFullPath(path)
		}

		// Get stdin data(pipe)
		if runtime.GOOS != "windows" {
			stdin := 0
			if !terminal.IsTerminal(stdin) {
				r.IsStdinPipe = true
			}
		}

		// create AuthMap
		r.CreateAuthMethodMap()

		err = pshell.Shell(r)
		return err
	}
	return app
}
