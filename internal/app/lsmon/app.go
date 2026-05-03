//go:build !windows

// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsmon

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	_ "net/http/pprof"

	"github.com/blacknon/lssh/internal/app/apputil"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	mon "github.com/blacknon/lssh/internal/monitor"
	"github.com/blacknon/lssh/internal/version"

	"github.com/urfave/cli"
)

func Lsmon() (app *cli.App) {
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
    # connect parallel ssh monitoring command
	lsmon
`

	// Create app
	app = cli.NewApp()
	app.Name = "lsmon"
	app.Usage = "TUI list select and parallel ssh monitoring command."
	app.Copyright = "blacknon(blacknon@orebibou.com)"
	app.Version = version.AppVersion("lsmon")

	// Set options
	app.Flags = []cli.Flag{
		// common option
		cli.StringSliceFlag{Name: "host,H", Usage: "connect `servername`."},
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config `filepath`."},
		cli.StringFlag{Name: "generate-lssh-conf", Usage: "print generated lssh config from OpenSSH config to stdout (`~/.ssh/config` by default)."},
		cli.StringFlag{Name: "logfile,L", Usage: "Set log file path."},
		cli.BoolFlag{Name: "share-connect,s", Usage: "reuse the monitor SSH connection for terminals."},
		cli.BoolFlag{Name: "localrc", Usage: "use local bashrc shell for opened terminals."},
		cli.BoolFlag{Name: "not-localrc", Usage: "not use local bashrc shell for opened terminals."},

		// Other bool
		cli.BoolFlag{Name: "list,l", Usage: "print server list from config."},
		cli.BoolFlag{Name: "debug", Usage: "debug pprof. use port 6060."},
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

		logpath := c.String("logfile")
		if logpath == "" {
			logpath = "/dev/null"
		}
		logpath = getAbsPath(logpath)

		logfile, err := setupLogOutput(logpath)
		if err != nil {
			return err
		}
		defer logfile.Close()

		hosts := c.StringSlice("host")
		confpath := c.String("file")
		controlMasterOverride, controlMasterErr := common.GetControlMasterOverride(c)
		if controlMasterErr != nil {
			return controlMasterErr
		}

		if handled, err := conf.HandleGenerateConfigMode(c.String("generate-lssh-conf"), os.Stdout); handled {
			return err
		}

		data, err := conf.ReadWithFallback(confpath, os.Stderr)
		if err != nil {
			return err
		}

		allNames := conf.GetNameList(data)
		names := append([]string(nil), allNames...)
		names, err = data.FilterServersByOperation(names, "sftp_transport")
		if err != nil {
			return err
		}
		sort.Strings(names)

		if c.Bool("list") {
			fmt.Fprintf(os.Stdout, "lssh Server List:\n")
			for v := range names {
				fmt.Fprintf(os.Stdout, "  %s\n", names[v])
			}
			return nil
		}

		selected, err := apputil.SelectOperationHosts(
			hosts,
			allNames,
			names,
			data,
			"sftp_transport",
			"No servers matched the current config conditions.",
			"Input Server does not support SFTP-based monitoring.",
			"lsmon>>",
			true,
			apputil.PromptServerSelection,
		)
		if err != nil {
			return err
		}

		r := buildRun(c, selected, controlMasterOverride, c.Bool("share-connect"))
		r.Conf = data
		r.Conf.Common.ConnectTimeout = 5

		startDebugServer(c.Bool("debug"))

		// create AuthMap
		r.CreateAuthMethodMap()

		return mon.Run(r)
	}
	return app
}

// getAbsPath return absolute path convert.
// Replace `~` with your home directory.
func getAbsPath(path string) string {
	// Replace home directory
	usr, _ := user.Current()
	path = strings.Replace(path, "~", usr.HomeDir, 1)

	path, _ = filepath.Abs(path)
	return path
}
