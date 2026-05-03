// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsftp

import (
	"fmt"
	"os"
	"sort"

	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/sftp"
	"github.com/blacknon/lssh/internal/version"
	"github.com/urfave/cli"
)

func Lsftp() (app *cli.App) {
	// Default config file path
	defConf := common.GetDefaultConfigPath()

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
	app.Version = version.AppVersion(app.Name)

	app.Flags = []cli.Flag{
		cli.StringSliceFlag{Name: "host,H", Usage: "connect `servername`"},
		cli.BoolFlag{Name: "list,l", Usage: "print server list from config"},
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config file path"},
		cli.StringFlag{Name: "generate-lssh-conf", Usage: "print generated lssh config from OpenSSH config to stdout (`~/.ssh/config` by default)."},
		cli.BoolFlag{Name: "auto-reconnect", Usage: "automatically reconnect disconnected hosts before commands"},
		cli.BoolFlag{Name: "help,h", Usage: "print this help"},
	}
	app.Flags = append(app.Flags, common.ControlMasterOverrideFlags()...)

	app.EnableBashCompletion = true
	app.HideHelp = true

	app.Action = func(c *cli.Context) error {
		if c.Bool("help") {
			cli.ShowAppHelp(c)
			return nil
		}

		controlMasterOverride, controlMasterErr := common.GetControlMasterOverride(c)
		if controlMasterErr != nil {
			return controlMasterErr
		}

		hosts := c.StringSlice("host")
		confpath := c.String("file")

		if handled, err := conf.HandleGenerateConfigMode(c.String("generate-lssh-conf"), os.Stdout); handled {
			return err
		}

		// Get config data
		data, err := conf.ReadWithFallback(confpath, os.Stderr)
		if err != nil {
			return err
		}

		// Get Server Name List (and sort List)
		allNames := conf.GetNameList(data)
		names := append([]string(nil), allNames...)
		names, err = data.FilterServersByOperation(names, "sftp_transport")
		if err != nil {
			return err
		}
		sort.Strings(names)

		if c.Bool("list") {
			fmt.Fprintln(os.Stdout, "lssh Server List:")
			for _, name := range names {
				fmt.Fprintf(os.Stdout, "  %s\n", name)
			}
			return nil
		}

		selected, err := selectSFTPServers(hosts, allNames, names, data, promptServerSelection)
		if err != nil {
			return err
		}

		// scp struct
		runSftp := new(sftp.RunSftp)
		runSftp.Config = data
		runSftp.SelectServer = selected
		runSftp.ControlMasterOverride = controlMasterOverride
		runSftp.AutoReconnect = c.Bool("auto-reconnect")

		// start lsftp shell
		runSftp.Start()
		return nil
	}

	return app
}
