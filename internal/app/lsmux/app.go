// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsmux

import (
	"os"
	"strings"

	applssh "github.com/blacknon/lssh/internal/app/lssh"
	"github.com/blacknon/lssh/internal/common"
	"github.com/blacknon/lssh/internal/version"
	"github.com/urfave/cli"
)

// Lsmux creates a compatibility wrapper app that forwards to `lssh -P`.
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

NOTE:
    lsmux is a compatibility wrapper for 'lssh -P'.
`

	app = cli.NewApp()
	app.Name = "lsmux"
	app.Usage = "Compatibility wrapper for the lssh mux UI (`lssh -P`)."
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
		cli.BoolFlag{Name: "localrc", Usage: "use local bashrc shell."},
		cli.BoolFlag{Name: "not-localrc", Usage: "not use local bashrc shell."},
		cli.StringFlag{Name: "session", Usage: "persistent mux session `name`."},
		cli.StringFlag{Name: "socket-path", Usage: "socket `path` for persistent mux session."},
		cli.BoolFlag{Name: "attach", Usage: "attach to an existing persistent mux session."},
		cli.BoolFlag{Name: "detach", Usage: "create or keep a persistent mux session without attaching."},
		cli.BoolFlag{Name: "list-sessions", Usage: "list persistent mux sessions."},
		cli.BoolFlag{Name: "kill-session", Usage: "kill the named persistent mux session."},
		cli.BoolFlag{Name: "enable-transfer", Usage: "enable file transfer UI even if disabled in config."},
		cli.BoolFlag{Name: "disable-transfer", Usage: "disable file transfer UI for this session."},
		cli.BoolFlag{Name: "list,l", Usage: "print server list from config."},
		cli.BoolFlag{Name: "help,h", Usage: "print this help"},
		cli.BoolFlag{Name: "mux-daemon", Hidden: true},
		cli.BoolFlag{Name: "mux-child", Hidden: true},
	}
	app.Flags = append(app.Flags, common.ControlMasterOverrideFlags()...)

	app.Action = func(c *cli.Context) error {
		if c.Bool("help") {
			cli.ShowAppHelp(c)
			return nil
		}

		return runCompatLsmux(TranslateCompatArgs(common.NormalizeGenerateLSSHConfArgs(os.Args)))
	}

	return app
}

func TranslateCompatArgs(args []string) []string {
	if len(args) == 0 {
		return []string{"lsmux", "-P"}
	}

	result := make([]string, 0, len(args)+1)
	result = append(result, args[0], "-P")

	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--session":
			result = append(result, "--mux-session")
		case strings.HasPrefix(arg, "--session="):
			result = append(result, "--mux-session="+strings.TrimPrefix(arg, "--session="))
		case arg == "--socket-path":
			result = append(result, "--mux-socket-path")
		case strings.HasPrefix(arg, "--socket-path="):
			result = append(result, "--mux-socket-path="+strings.TrimPrefix(arg, "--socket-path="))
		case arg == "--attach":
			result = append(result, "--mux-attach")
		case arg == "--detach":
			result = append(result, "--mux-detach")
		case arg == "--list-sessions":
			result = append(result, "--mux-list-sessions")
		case arg == "--kill-session":
			result = append(result, "--mux-kill-session")
		default:
			result = append(result, arg)
		}
	}

	return result
}

func runCompatLsmux(args []string) error {
	origArgs := os.Args
	os.Args = append([]string(nil), args...)
	defer func() {
		os.Args = origArgs
	}()

	app := applssh.Lssh()
	return app.Run(common.ParseArgs(app.Flags, args))
}
