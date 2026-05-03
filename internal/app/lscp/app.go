// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lscp

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/blacknon/lssh/internal/check"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/scp"
	"github.com/blacknon/lssh/internal/version"
	"github.com/urfave/cli"
)

func Lscp() (app *cli.App) {
	// Default config file path
	defConf := common.GetDefaultConfigPath()

	// Set help templete
	cli.AppHelpTemplate = `NAME:
    {{.Name}} - {{.Usage}}
USAGE:
    {{.HelpName}} {{if .VisibleFlags}}[options]{{end}} (local|remote):from_path... (local|remote):to_path
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
    # local to remote scp
    {{.Name}} /path/to/local... remote:/path/to/remote

    # remote to local scp
    {{.Name}} remote:/path/to/remote... /path/to/local

    # remote to remote scp
    {{.Name}} remote:/path/to/remote... remote:/path/to/local
`
	// Create app
	app = cli.NewApp()
	// app.UseShortOptionHandling = true
	app.Name = "lscp"
	app.Usage = "TUI list select and parallel scp client command."
	app.Copyright = "blacknon(blacknon@orebibou.com)"
	app.Version = version.AppVersion(app.Name)

	app.Flags = []cli.Flag{
		cli.StringSliceFlag{Name: "host,H", Usage: "connect servernames"},
		cli.BoolFlag{Name: "list,l", Usage: "print server list from config"},
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config file path"},
		cli.StringFlag{Name: "generate-lssh-conf", Usage: "print generated lssh config from OpenSSH config to stdout (`~/.ssh/config` by default)."},
		cli.IntFlag{Name: "parallel,P", Value: 1, Usage: "parallel file copy count per host"},
		cli.BoolFlag{Name: "permission,p", Usage: "copy file permission"},
		cli.BoolFlag{Name: "dry-run", Usage: "show copy actions without modifying files"},
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

		hosts := c.StringSlice("host")
		confpath := c.String("file")
		controlMasterOverride, controlMasterErr := common.GetControlMasterOverride(c)
		if controlMasterErr != nil {
			return controlMasterErr
		}

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

		if len(c.Args()) < 2 {
			return fmt.Errorf("Too few arguments.")
		}

		// Set args path
		fromArgs := c.Args()[:c.NArg()-1]
		toArg := c.Args()[c.NArg()-1]

		isFromInRemote := false
		isFromInLocal := false
		for _, from := range fromArgs {
			isFromRemote, _, err := check.ParseScpPathE(from)
			if err != nil {
				return err
			}

			if isFromRemote {
				isFromInRemote = true
			} else {
				isFromInLocal = true
			}
		}
		isToRemote, _, err := check.ParseScpPathE(toArg)
		if err != nil {
			return err
		}

		if err := check.ValidateCopyTypes(isFromInRemote, isFromInLocal, isToRemote, len(hosts)); err != nil {
			return err
		}

		fromServer, toServer, err := selectSCPServers(hosts, allNames, names, data, isFromInRemote, isToRemote)
		if err != nil {
			return err
		}

		// scp struct
		scp := new(scp.Scp)
		scp.ControlMasterOverride = controlMasterOverride

		// set from info
		for _, from := range fromArgs {
			isFromRemote, fromPath, err := check.ParseScpPathE(from)
			if err != nil {
				return err
			}
			displayFromPath := fromPath

			// Check local file exisits
			if !isFromRemote {
				_, err := os.Stat(common.GetFullPath(fromPath))
				if err != nil {
					fmt.Fprintf(os.Stderr, "not found path %s \n", fromPath)
					os.Exit(1)
				}
				fromPath = common.GetFullPath(fromPath)
			}

			// set from data
			scp.From.IsRemote = isFromRemote
			if isFromRemote {
				fromPath = check.EscapePath(fromPath)
			}
			scp.From.Path = append(scp.From.Path, fromPath)
			scp.From.DisplayPath = append(scp.From.DisplayPath, displayFromPath)
		}
		scp.From.Server = fromServer

		isToRemote, toPath, err := check.ParseScpPathE(toArg)
		if err != nil {
			return err
		}
		scp.To.IsRemote = isToRemote
		if isToRemote {
			toPath = check.EscapePath(toPath)
		}
		scp.To.Path = []string{toPath}
		scp.To.Server = toServer

		scp.Parallel = c.Int("parallel") > 1
		scp.ParallelNum = c.Int("parallel")
		scp.Permission = c.Bool("permission")
		scp.DryRun = c.Bool("dry-run")
		scp.Config = data

		// print from
		if !isFromInRemote {
			fmt.Fprintf(os.Stderr, "From local:%s\n", scp.From.DisplayPath)
		} else {
			fmt.Fprintf(os.Stderr, "From remote(%s):%s\n", strings.Join(scp.From.Server, ","), scp.From.Path)
		}

		// print to
		if !isToRemote {
			fmt.Fprintf(os.Stderr, "To   local:%s\n", scp.To.Path)
		} else {
			fmt.Fprintf(os.Stderr, "To   remote(%s):%s\n", strings.Join(scp.To.Server, ","), scp.To.Path)
		}

		scp.Start()
		return nil
	}

	return app
}
