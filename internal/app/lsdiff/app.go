// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsdiff

import (
	"fmt"
	"os"

	"github.com/blacknon/lssh/internal/app/apputil"
	"github.com/blacknon/lssh/internal/check"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	diffapp "github.com/blacknon/lssh/internal/lsdiff"
	"github.com/blacknon/lssh/internal/version"
	"github.com/urfave/cli"
)

var (
	resolveTargetsFn = diffapp.ResolveTargets
	fetchDocumentsFn = diffapp.FetchDocuments
	alignDocumentsFn = diffapp.AlignDocuments
	newViewerFn      = diffapp.NewViewer
)

func Lsdiff() (app *cli.App) {
	defConf := common.GetDefaultConfigPath()

	cli.AppHelpTemplate = `NAME:
    {{.Name}} - {{.Usage}}
USAGE:
    {{.HelpName}} {{if .VisibleFlags}}[options]{{end}} remote_path | @host:/path...
    {{if len .Authors}}
AUTHOR:
    {{range .Authors}}{{ . }}{{end}}
    {{end}}{{if .VisibleFlags}}
OPTIONS:
    {{range .VisibleFlags}}{{.}}
    {{end}}{{end}}{{if .Version}}
VERSION:
    {{.Version}}
    {{end}}
USAGE:
    # select multiple hosts and compare the same remote path
    {{.Name}} /etc/hosts

    # compare different paths on specific hosts
    {{.Name}} @host1:/etc/hosts @host2:/etc/hosts @host3:/tmp/hosts
`

	app = cli.NewApp()
	app.Name = "lsdiff"
	app.Usage = "Compare remote files from multiple SSH hosts in a synchronized TUI."
	app.Copyright = "blacknon(blacknon@orebibou.com)"
	app.Version = version.AppVersion(app.Name)
	app.EnableBashCompletion = true
	app.HideHelp = true
	app.Flags = []cli.Flag{
		cli.StringSliceFlag{Name: "host,H", Usage: "connect `servername`."},
		cli.BoolFlag{Name: "list,l", Usage: "print server list from config."},
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config `filepath`."},
		cli.StringFlag{Name: "generate-lssh-conf", Usage: "print generated lssh config from OpenSSH config to stdout (`~/.ssh/config` by default)."},
		cli.BoolFlag{Name: "help,h", Usage: "print this help"},
	}
	app.Flags = append(app.Flags, common.ControlMasterOverrideFlags()...)

	app.Action = func(c *cli.Context) error {
		if c.Bool("help") {
			cli.ShowAppHelp(c)
			return nil
		}

		config, handled, err := apputil.LoadConfigWithGenerateMode(c, os.Stdout, os.Stderr)
		if handled {
			return err
		}

		controlMasterOverride, controlMasterErr := common.GetControlMasterOverride(c)
		if controlMasterErr != nil {
			return controlMasterErr
		}
		if err != nil {
			return err
		}

		allNames, names, err := apputil.SortedServerNames(config, "sftp_transport")
		if err != nil {
			return err
		}

		if c.Bool("list") {
			apputil.PrintServerList(os.Stdout, names)
			return nil
		}

		if c.NArg() == 0 {
			cli.ShowAppHelp(c)
			return fmt.Errorf("lsdiff requires at least one remote path")
		}

		targets, err := resolveDiffTargets(config, allNames, names, c.StringSlice("host"), c.Args())
		if err != nil {
			return err
		}

		documents, err := fetchDocumentsFn(config, targets, controlMasterOverride)
		if err != nil {
			return err
		}
		if len(documents) < 2 {
			return fmt.Errorf("lsdiff requires at least two files to compare")
		}

		viewer := newViewerFn(alignDocumentsFn(documents))
		return viewer.Run()
	}

	return app
}

func resolveDiffTargets(config conf.Config, allNames, supportedNames, flagHosts, args []string) ([]diffapp.Target, error) {
	if len(flagHosts) == 0 {
		targets, err := resolveTargetsFn(config, args)
		if err != nil {
			return nil, err
		}
		if !check.ExistServer(targetHosts(targets), supportedNames) {
			if !check.ExistServer(targetHosts(targets), allNames) {
				return nil, fmt.Errorf("selected host not found from list")
			}
			return nil, fmt.Errorf("selected host does not support SFTP-based transfer")
		}
		return targets, nil
	}

	if len(args) != 1 {
		return nil, fmt.Errorf("--host can only be used with a single common remote path")
	}

	target, err := diffapp.ParseTargetSpecWithHosts(args[0], allNames)
	if err != nil {
		return nil, err
	}
	if target.Host != "" {
		return nil, fmt.Errorf("--host cannot be used with explicit @host:/path targets")
	}
	if len(flagHosts) < 2 {
		return nil, fmt.Errorf("select at least two hosts")
	}
	selected, err := apputil.SelectOperationHosts(
		flagHosts,
		allNames,
		supportedNames,
		config,
		"sftp_transport",
		"no servers matched the current config conditions",
		"selected host does not support SFTP-based transfer",
		"lsdiff>>",
		true,
		apputil.PromptServerSelection,
	)
	if err != nil {
		if err.Error() == "Input Server not found from list." {
			return nil, fmt.Errorf("selected host not found from list")
		}
		return nil, err
	}

	targets := make([]diffapp.Target, 0, len(selected))
	for _, host := range selected {
		targets = append(targets, diffapp.Target{
			Host:       host,
			RemotePath: target.RemotePath,
			Title:      host + ":" + target.RemotePath,
		})
	}
	return targets, nil
}

func targetHosts(targets []diffapp.Target) []string {
	hosts := make([]string, 0, len(targets))
	for _, target := range targets {
		hosts = append(hosts, target.Host)
	}
	return hosts
}
