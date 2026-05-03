// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lssh

import (
	"fmt"
	"os"
	"strings"

	"github.com/blacknon/lssh/internal/common"
	"github.com/blacknon/lssh/internal/core/apputil"
	"github.com/blacknon/lssh/internal/version"
	"github.com/urfave/cli"
)

func resolveTunnelOption(goos, spec string) (enabled bool, local, remote int, err error) {
	if spec == "" {
		return false, 0, 0, nil
	}

	if goos == "windows" {
		return false, 0, 0, fmt.Errorf("--tunnel is not supported on Windows")
	}

	local, remote, err = common.ParseTunnelSpec(spec)
	if err != nil {
		return false, 0, 0, fmt.Errorf("invalid --tunnel format: %w", err)
	}

	return true, local, remote, nil
}

func Lssh() (app *cli.App) {
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
    # connect ssh
    {{.Name}}

    # run command selected server over ssh.
    {{.Name}} command...

    # run command parallel in selected server over ssh.
    {{.Name}} -p command...

    # run command or shell in mux UI.
    {{.Name}} -P [command...]
`

	// Create app
	app = cli.NewApp()
	// app.UseShortOptionHandling = true
	app.Name = "lssh"
	app.Usage = "TUI list select and parallel ssh client command."
	app.Copyright = "blacknon(blacknon@orebibou.com)"
	app.Version = version.AppVersion(app.Name)

	// TODO(blacknon): オプションの追加
	//     -T       ... マウント・リバースマウントのTypeを指定できるようにする(v0.7.0)
	//                  ※ smb/nfsの指定
	// TODO(blacknon): オプションの追加
	//     -f       ... バックグラウンドでの接続(X11接続やport forwardingをバックグラウンドで実行する場合など)。
	//                  「ssh -f」と同じ。 (v0.7.0)
	//                  (https://github.com/sevlyar/go-daemon)
	// TODO(blacknon): オプションの追加
	//     --read_profile
	//              ... デフォルトではlocalrc読み込みでのshellではsshサーバ上のprofileは読み込まないが、このオプションを指定することで読み込まれるようになる (v0.7.0)
	// TODO(blacknon): オプションの追加
	//     -P
	//              ... Terminal multipluxerを用いたParallel Shell/Command実行を有効にする(v0.7.0)
	//                  ※ -pとの排他制御が必要

	// Set options
	app.Flags = []cli.Flag{
		// common option
		cli.StringSliceFlag{Name: "host,H", Usage: "connect `servername`."},
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config `filepath`."},
		cli.StringFlag{Name: "generate-lssh-conf", Usage: "print generated lssh config from OpenSSH config to stdout (`~/.ssh/config` by default)."},

		// port forward (with dynamic forward) option
		cli.StringSliceFlag{Name: "L", Usage: "Local port forward mode.Specify a `[bind_address:]port:remote_address:port`. Only single connection works."},
		cli.StringSliceFlag{Name: "R", Usage: "Remote port forward mode.Specify a `[bind_address:]port:remote_address:port`. If only one port is specified, it will operate as Reverse Dynamic Forward. Only single connection works."},
		cli.StringFlag{Name: "D", Usage: "Dynamic port forward mode(Socks5). Specify a `port`. Only single connection works."},
		cli.StringFlag{Name: "d", Usage: "HTTP Dynamic port forward mode. Specify a `port`. Only single connection works."},
		cli.StringFlag{Name: "r", Usage: "HTTP Reverse Dynamic port forward mode. Specify a `port`. Only single connection works."},
		cli.StringFlag{Name: "M", Usage: "NFS Dynamic forward mode. Specify a `port:/path/to/remote`. Only single connection works."},
		cli.StringFlag{Name: "m", Usage: "NFS Reverse Dynamic forward mode. Specify a `port:/path/to/local`. Only single connection works."},
		cli.StringFlag{Name: "S", Usage: "SMB Dynamic forward mode. Specify a `port:/path/to/remote`. Only single connection works."},
		cli.StringFlag{Name: "s", Usage: "SMB Reverse Dynamic forward mode. Specify a `port:/path/to/local`. Only single connection works."},
		// tunnel device option (like `ssh -w local:remote`)
		cli.StringFlag{Name: "tunnel", Usage: "Enable tunnel device. Specify `${local}:${remote}` (use 'any' to request next available)."},

		// Other bool
		cli.BoolFlag{Name: "w", Usage: "Displays the server header when in command execution mode."},
		cli.BoolFlag{Name: "W", Usage: "Not displays the server header when in command execution mode."},
		cli.BoolFlag{Name: "not-execute,N", Usage: "not execute remote command and shell."},
		cli.BoolFlag{Name: "X11,X", Usage: "Enable x11 forwarding(forward to ${DISPLAY})."},
		cli.BoolFlag{Name: "Y", Usage: "Enable trusted x11 forwarding(forward to ${DISPLAY})."},
		cli.BoolFlag{Name: "term,t", Usage: "run specified command at terminal."},
		cli.BoolFlag{Name: "parallel,p", Usage: "run command parallel node(tail -F etc...)."},
		cli.BoolFlag{Name: "P", Usage: "run shell or command in mux UI (lsmux compatible)."},
		cli.BoolFlag{Name: "hold", Usage: "keep command panes after remote command exits (with -P)."},
		cli.BoolFlag{Name: "allow-layout-change", Usage: "allow opening new pages/panes even in command mode (with -P)."},
		cli.StringFlag{Name: "mux-session", Usage: "persistent mux session `name` (with -P)."},
		cli.StringFlag{Name: "mux-socket-path", Usage: "socket `path` for persistent mux session (with -P)."},
		cli.BoolFlag{Name: "mux-attach", Usage: "attach to an existing mux session (with -P)."},
		cli.BoolFlag{Name: "mux-detach", Usage: "create or keep a persistent mux session without attaching (with -P)."},
		cli.BoolFlag{Name: "mux-list-sessions", Usage: "list persistent mux sessions (with -P)."},
		cli.BoolFlag{Name: "mux-kill-session", Usage: "kill the named mux session (with -P)."},
		cli.StringFlag{Name: "attach", Usage: "attach to an existing connector session by `session-id`."},
		cli.BoolFlag{Name: "detach", Usage: "start a connector shell session without attaching when supported."},
		cli.BoolFlag{Name: "localrc", Usage: "use local bashrc shell."},
		cli.BoolFlag{Name: "not-localrc", Usage: "not use local bashrc shell."},
		cli.BoolFlag{Name: "enable-transfer", Usage: "enable file transfer UI in mux mode even if disabled in config."},
		cli.BoolFlag{Name: "disable-transfer", Usage: "disable file transfer UI in mux mode."},
		cli.BoolFlag{Name: "list,l", Usage: "print server list from config."},
		cli.BoolFlag{Name: "help,h", Usage: "print this help"},
		// Background (like ssh -f)
		cli.BoolFlag{Name: "f", Usage: "Run in background after forwarding/connection (ssh -f like)."},
		cli.BoolFlag{Name: "mux-daemon", Hidden: true},
		cli.BoolFlag{Name: "mux-child", Hidden: true},
	}
	app.Flags = append(app.Flags, common.ControlMasterOverrideFlags()...)
	app.EnableBashCompletion = true
	app.HideHelp = true

	// Run command action
	app.Action = func(c *cli.Context) error {
		// show help messages
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

		// Set `exec command` or `shell` flag
		isMulti := false
		if len(c.Args()) > 0 && !c.Bool("not-execute") {
			isMulti = true
		}

		// Extraction server name list from 'data'
		_, names, err := apputil.SortedServerNames(data, "")
		if err != nil {
			return err
		}

		// Check list flag
		if c.Bool("list") {
			apputil.PrintServerList(os.Stdout, names)
			return nil
		}

		enableX11 := c.Bool("X11")
		enableTrustedX11 := c.Bool("Y")
		connectorAttachSession := c.String("attach")
		connectorDetach := c.Bool("detach")
		muxSessionName := c.String("mux-session")
		muxSocketPath := c.String("mux-socket-path")
		if strings.TrimSpace(muxSocketPath) == "" {
			muxSocketPath = data.Mux.SocketPath
		}

		if c.Bool("P") {
			return runMuxMode(c, data, names, hosts, c.String("file"), controlMasterOverride, enableX11, enableTrustedX11, connectorAttachSession, connectorDetach, muxSessionName, muxSocketPath)
		}

		selected, err := selectServers(hosts, names, data, isMulti, c.Bool("f"), promptServerSelection)
		if err != nil {
			return err
		}

		if err := validateConnectorShellOptions(connectorFlagOptions{
			AttachSession:            connectorAttachSession,
			Detach:                   connectorDetach,
			CommandArgs:              append([]string(nil), c.Args()...),
			MuxMode:                  c.Bool("P"),
			ParallelMode:             c.Bool("parallel"),
			TermMode:                 c.Bool("term"),
			NotExecute:               c.Bool("not-execute"),
			LocalRc:                  c.Bool("localrc"),
			NotLocalRc:               c.Bool("not-localrc"),
			X11:                      enableX11,
			X11Trusted:               enableTrustedX11,
			Background:               c.Bool("f"),
			LocalForwards:            len(c.StringSlice("L")),
			RemoteForwards:           len(c.StringSlice("R")),
			DynamicForward:           c.String("D") != "",
			HTTPDynamicForward:       c.String("d") != "",
			HTTPReverseForward:       c.String("r") != "",
			NFSDynamicForward:        c.String("M") != "",
			NFSReverseDynamicForward: c.String("m") != "",
			SMBDynamicForward:        c.String("S") != "",
			SMBReverseDynamicForward: c.String("s") != "",
			Tunnel:                   c.String("tunnel") != "",
		}, selected, data); err != nil {
			return err
		}

		r, err := buildRun(c, data, selected, controlMasterOverride, connectorAttachSession, connectorDetach, enableX11, enableTrustedX11)
		if err != nil {
			return err
		}

		if c.Bool("f") && os.Getenv("_LSSH_DAEMON") != "1" {
			return runBackgroundLssh(selected, hosts)
		}

		r.Start()
		return nil
	}
	return app
}
