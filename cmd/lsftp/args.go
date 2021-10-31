// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"os/user"
	"sort"

	"github.com/blacknon/lssh/conf"
	"github.com/blacknon/lssh/list"
	"github.com/blacknon/lssh/sftp"
	"github.com/urfave/cli"
)

func Lsftp() (app *cli.App) {
	// Default config file path
	usr, _ := user.Current()
	defConf := usr.HomeDir + "/.lssh.conf"

	// Set help templete
	cli.AppHelpTemplate = `NAME:
    {{.Name}} - {{.Usage}}
USAGE:
    {{.HelpName}} {{if .VisibleFlags}}[options]{{end}}
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
	# start lsftp shell
	{{.Name}}
`
	// Create app
	app = cli.NewApp()
	// app.UseShortOptionHandling = true
	app.Name = "lsftp"
	app.Usage = "TUI list select and parallel sftp client command."
	app.Copyright = "blacknon(blacknon@orebibou.com)"
	app.Version = "0.6.2"

	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config file path"},
		cli.BoolFlag{Name: "help,h", Usage: "print this help"},
	}

	app.EnableBashCompletion = true
	app.HideHelp = true

	app.Action = func(c *cli.Context) error {
		// show help messages
		if c.Bool("help") {
			cli.ShowAppHelp(c)
			os.Exit(0)
		}

		// hosts := c.StringSlice("host")
		confpath := c.String("file")

		// Get config data
		data := conf.ReadConf(confpath)

		// Get Server Name List (and sort List)
		names := conf.GetNameList(data)
		sort.Strings(names)

		// create select list
		l := new(list.ListInfo)
		l.Prompt = "lsftp>>"
		l.NameList = names
		l.DataList = data
		l.MultiFlag = true
		l.View()

		// selected check
		selected := l.SelectName
		if selected[0] == "ServerName" {
			fmt.Fprintln(os.Stderr, "Server not selected.")
			os.Exit(1)
		}

		// scp struct
		runSftp := new(sftp.RunSftp)
		runSftp.Config = data
		runSftp.SelectServer = selected

		// start lsftp shell
		runSftp.Start()
		return nil
	}

	return app
}
