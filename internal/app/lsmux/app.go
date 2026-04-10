// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsmux

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"sort"

	"github.com/blacknon/lssh/internal/check"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/mux"
	"github.com/blacknon/lssh/internal/version"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
)

// Lsmux creates the lsmux CLI app.
func Lsmux() (app *cli.App) {
	defConf := common.GetDefaultConfigPath()

	cli.AppHelpTemplate = `NAME:
    {{.Name}} - {{.Usage}}
USAGE:
    {{.HelpName}} {{if .VisibleFlags}}[options]{{end}} [command...]
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
    lsmux
    lsmux command...
`

	app = cli.NewApp()
	app.Name = "lsmux"
	app.Usage = "TUI mux style SSH client with host selector and pane management."
	app.Copyright = "blacknon(blacknon@orebibou.com)"
	app.Version = version.AppVersion(app.Name)
	app.EnableBashCompletion = true
	app.HideHelp = true
	app.Flags = []cli.Flag{
		cli.StringSliceFlag{Name: "host,H", Usage: "connect `servername`."},
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config `filepath`."},
		cli.StringFlag{Name: "generate-lssh-conf", Usage: "print generated lssh config from OpenSSH config to stdout (`~/.ssh/config` by default)."},
		cli.StringSliceFlag{Name: "R", Usage: "Remote port forward mode.Specify a `[bind_address:]port:remote_address:port`. If only one port is specified, it will operate as Reverse Dynamic Forward."},
		cli.StringFlag{Name: "r", Usage: "HTTP Reverse Dynamic port forward mode. Specify a `port`."},
		cli.StringFlag{Name: "m", Usage: "NFS Reverse Dynamic forward mode. Specify a `port:/path/to/local`."},
		cli.BoolFlag{Name: "hold", Usage: "keep command panes after remote command exits."},
		cli.BoolFlag{Name: "allow-layout-change", Usage: "allow opening new pages/panes even in command mode."},
		cli.BoolFlag{Name: "list,l", Usage: "print server list from config."},
		cli.BoolFlag{Name: "help,h", Usage: "print this help"},
	}

	app.Action = func(c *cli.Context) error {
		if c.Bool("help") {
			cli.ShowAppHelp(c)
			os.Exit(0)
		}

		if handled, err := conf.HandleGenerateConfigMode(c.String("generate-lssh-conf"), os.Stdout); handled {
			return err
		}

		data, err := conf.ReadWithFallback(c.String("file"), os.Stderr)
		if err != nil {
			return err
		}
		names := conf.GetNameList(data)
		sort.Strings(names)

		if c.Bool("list") {
			fmt.Fprintln(os.Stdout, "lssh Server List:")
			for _, name := range names {
				fmt.Fprintf(os.Stdout, "  %s\n", name)
			}
			return nil
		}

		initialHosts := c.StringSlice("host")
		if len(initialHosts) > 0 && !check.ExistServer(initialHosts, names) {
			return fmt.Errorf("input server not found from list")
		}
		forwardConfig := mux.SessionOptions{}

		var forwards []*conf.PortForward
		for _, forwardargs := range c.StringSlice("R") {
			f := new(conf.PortForward)
			f.Mode = "R"

			if regexp.MustCompile(`^[0-9]+$`).Match([]byte(forwardargs)) {
				forwardConfig.ReverseDynamicPortForward = forwardargs
				continue
			}

			f.Local, f.Remote, err = common.ParseForwardPort(forwardargs)
			if err != nil {
				return err
			}
			forwards = append(forwards, f)
		}
		forwardConfig.PortForward = forwards
		forwardConfig.HTTPReverseDynamicPortForward = c.String("r")
		if nfsReverseForwarding := c.String("m"); nfsReverseForwarding != "" {
			port, path, err := common.ParseNFSForwardPortPath(nfsReverseForwarding)
			if err != nil {
				return err
			}
			forwardConfig.NFSReverseDynamicForwardPort = port
			forwardConfig.NFSReverseDynamicForwardPath = common.GetFullPath(path)
		}

		var (
			stdinData []byte
			readErr   error
		)
		if len(c.Args()) > 0 && runtime.GOOS != "windows" {
			stdin := 0
			if !terminal.IsTerminal(stdin) {
				stdinData, readErr = io.ReadAll(os.Stdin)
				if readErr != nil {
					return readErr
				}
			}
		}

		manager, err := mux.NewManager(data, names, c.Args(), stdinData, initialHosts, c.Bool("hold"), c.Bool("allow-layout-change"), forwardConfig)
		if err != nil {
			return err
		}
		return manager.Run()
	}

	return app
}
