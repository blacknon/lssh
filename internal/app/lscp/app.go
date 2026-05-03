// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lscp

import (
	"fmt"
	"os"
	"strings"

	"github.com/blacknon/lssh/internal/app/apputil"
	"github.com/blacknon/lssh/internal/check"
	"github.com/blacknon/lssh/internal/common"
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
		controlMasterOverride, controlMasterErr := common.GetControlMasterOverride(c)
		if controlMasterErr != nil {
			return controlMasterErr
		}

		data, handled, err := apputil.LoadConfigWithGenerateMode(c, os.Stdout, os.Stderr)
		if handled {
			return err
		}
		if err != nil {
			return err
		}

		allNames, names, err := apputil.SortedServerNames(data, "sftp_transport")
		if err != nil {
			return err
		}

		if c.Bool("list") {
			apputil.PrintServerList(os.Stdout, names)
			return nil
		}

		if len(c.Args()) < 2 {
			return fmt.Errorf("Too few arguments.")
		}

		// Set args path
		fromArgs := c.Args()[:c.NArg()-1]
		toArg := c.Args()[c.NArg()-1]

		fromSpecs := make([]apputil.TransferPathSpec, 0, len(fromArgs))
		isFromInRemote := false
		isFromInLocal := false
		for _, from := range fromArgs {
			spec, err := apputil.ParseTransferPathSpec(from)
			if err != nil {
				return err
			}
			fromSpecs = append(fromSpecs, spec)

			if spec.IsRemote {
				isFromInRemote = true
			} else {
				isFromInLocal = true
			}
		}
		toSpec, err := apputil.ParseTransferPathSpec(toArg)
		if err != nil {
			return err
		}

		if err := check.ValidateCopyTypes(isFromInRemote, isFromInLocal, toSpec.IsRemote, len(hosts)); err != nil {
			return err
		}

		fromServer, toServer, err := selectSCPServers(hosts, allNames, names, data, isFromInRemote, toSpec.IsRemote)
		if err != nil {
			return err
		}
		preparedFrom, err := apputil.PrepareTransferSourcePaths(fromSpecs)
		if err != nil {
			return err
		}
		preparedTo, err := apputil.PrepareTransferDestinationPath(toSpec)
		if err != nil {
			return err
		}

		// scp struct
		scp := new(scp.Scp)
		scp.ControlMasterOverride = controlMasterOverride

		// set from info
		for _, from := range preparedFrom {
			scp.From.IsRemote = from.IsRemote
			scp.From.Path = append(scp.From.Path, from.Path)
			scp.From.DisplayPath = append(scp.From.DisplayPath, from.DisplayPath)
		}
		scp.From.Server = fromServer

		scp.To.IsRemote = preparedTo.IsRemote
		scp.To.Path = []string{preparedTo.Path}
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
		if !toSpec.IsRemote {
			fmt.Fprintf(os.Stderr, "To   local:%s\n", scp.To.Path)
		} else {
			fmt.Fprintf(os.Stderr, "To   remote(%s):%s\n", strings.Join(scp.To.Server, ","), scp.To.Path)
		}

		scp.Start()
		return nil
	}

	return app
}
