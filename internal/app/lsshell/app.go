// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsshell

import (
	"os"

	"github.com/blacknon/lssh/internal/common"
	"github.com/blacknon/lssh/internal/core/apputil"
	pshell "github.com/blacknon/lssh/internal/pshell"
	"github.com/blacknon/lssh/internal/version"

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
	app.Flags = append(app.Flags, common.ControlMasterOverrideFlags()...)
	app.EnableBashCompletion = true
	app.HideHelp = true

	// Run command action
	app.Action = func(c *cli.Context) error {
		if c.Bool("help") {
			cli.ShowAppHelp(c)
			return nil
		}

		hosts := c.StringSlice("host")
		controlMasterOverride, controlMasterErr := common.GetControlMasterOverride(c)
		if controlMasterErr != nil {
			return controlMasterErr
		}

		data, handled, configErr := apputil.LoadConfigWithGenerateMode(c, os.Stdout, os.Stderr)
		if handled {
			return configErr
		}
		if configErr != nil {
			return configErr
		}

		_, names, err := apputil.SortedServerNames(data, "")
		if err != nil {
			return err
		}

		if c.Bool("list") {
			apputil.PrintServerList(os.Stdout, names)
			return nil
		}

		selected, err := selectServers(hosts, names, data, true, promptServerSelection)
		if err != nil {
			return err
		}

		r, err := buildRun(c, data, selected, controlMasterOverride)
		if err != nil {
			return err
		}
		r.CreateAuthMethodMap()

		return pshell.Shell(r)
	}
	return app
}
