// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsdiff

import (
	"fmt"
	"os"

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
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config `filepath`."},
		cli.StringFlag{Name: "generate-lssh-conf", Usage: "print generated lssh config from OpenSSH config to stdout (`~/.ssh/config` by default)."},
		cli.BoolFlag{Name: "help,h", Usage: "print this help"},
	}
	app.Flags = append(app.Flags, common.ControlMasterOverrideFlags()...)

	app.Action = func(c *cli.Context) error {
		if c.Bool("help") {
			cli.ShowAppHelp(c)
			os.Exit(0)
		}

		if c.NArg() == 0 {
			cli.ShowAppHelp(c)
			return fmt.Errorf("lsdiff requires at least one remote path")
		}

		if handled, err := conf.HandleGenerateConfigMode(c.String("generate-lssh-conf"), os.Stdout); handled {
			return err
		}

		controlMasterOverride, controlMasterErr := common.GetControlMasterOverride(c)
		if controlMasterErr != nil {
			return controlMasterErr
		}

		config, err := conf.ReadWithFallback(c.String("file"), os.Stderr)
		if err != nil {
			return err
		}

		targets, err := resolveTargetsFn(config, c.Args())
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
