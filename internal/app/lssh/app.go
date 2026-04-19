// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lssh

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/blacknon/lssh/internal/check"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/list"
	lsmuxsession "github.com/blacknon/lssh/internal/lsmuxsession"
	"github.com/blacknon/lssh/internal/mux"
	sshcmd "github.com/blacknon/lssh/internal/ssh"
	"github.com/blacknon/lssh/internal/version"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
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
			os.Exit(0)
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
		data, configErr := conf.ReadWithFallback(confpath, os.Stderr)
		if configErr != nil {
			return configErr
		}

		// Set `exec command` or `shell` flag
		isMulti := false
		if (len(c.Args()) > 0 || c.Bool("pshell")) && !c.Bool("not-execute") {
			isMulti = true
		}

		// Extraction server name list from 'data'
		names := conf.GetNameList(data)
		sort.Strings(names)

		// Check list flag
		if c.Bool("list") {
			fmt.Fprintf(os.Stdout, "lssh Server List:\n")
			for v := range names {
				fmt.Fprintf(os.Stdout, "  %s\n", names[v])
			}
			os.Exit(0)
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
			if c.Bool("mux-list-sessions") {
				return listLsshMuxSessions()
			}
			if c.Bool("mux-kill-session") {
				return killLsshMuxSession(muxSessionName)
			}
			if c.Bool("mux-attach") {
				return attachLsshMuxSession(muxSessionName, data)
			}
			if connectorAttachSession != "" || connectorDetach {
				return fmt.Errorf("--attach/--detach cannot be used with -P")
			}
			if c.Bool("enable-transfer") && c.Bool("disable-transfer") {
				return fmt.Errorf("--enable-transfer and --disable-transfer cannot be used together")
			}
			if len(hosts) > 0 && !check.ExistServer(hosts, names) {
				fmt.Fprintln(os.Stderr, "Input Server not found from list.")
				os.Exit(1)
			}

			var (
				err       error
				stdinData []byte
				forwards  []*conf.PortForward
			)

			for _, forwardargs := range c.StringSlice("L") {
				f := new(conf.PortForward)
				f.Mode = "L"
				f.LocalNetwork, f.Local, f.RemoteNetwork, f.Remote, err = common.ParseForwardSpec(forwardargs)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err)
					os.Exit(1)
				}
				forwards = append(forwards, f)
			}

			forwardConfig := mux.SessionOptions{
				PortForward: forwards,
				X11:         enableX11 || enableTrustedX11,
				X11Trusted:  enableTrustedX11,
				IsBashrc:    c.Bool("localrc"),
				IsNotBashrc: c.Bool("not-localrc"),
			}
			if c.Bool("enable-transfer") {
				enabled := true
				forwardConfig.TransferEnabled = &enabled
			}
			if c.Bool("disable-transfer") {
				enabled := false
				forwardConfig.TransferEnabled = &enabled
			}
			for _, forwardargs := range c.StringSlice("R") {
				f := new(conf.PortForward)
				f.Mode = "R"

				if regexp.MustCompile(`^[0-9]+$`).Match([]byte(forwardargs)) {
					forwardConfig.ReverseDynamicPortForward = forwardargs
					continue
				}

				f.Local, f.Remote, err = common.ParseForwardPort(forwardargs)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err)
					os.Exit(1)
				}
				forwards = append(forwards, f)
			}
			forwardConfig.PortForward = forwards
			forwardConfig.HTTPReverseDynamicPortForward = c.String("r")
			if nfsReverseForwarding := c.String("m"); nfsReverseForwarding != "" {
				port, path, parseErr := common.ParseNFSForwardPortPath(nfsReverseForwarding)
				if parseErr != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", parseErr)
					os.Exit(1)
				}
				forwardConfig.NFSReverseDynamicForwardPort = port
				forwardConfig.NFSReverseDynamicForwardPath = common.GetFullPath(path)
			}
			if smbReverseForwarding := c.String("s"); smbReverseForwarding != "" {
				port, path, parseErr := common.ParseNFSForwardPortPath(smbReverseForwarding)
				if parseErr != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", parseErr)
					os.Exit(1)
				}
				forwardConfig.SMBReverseDynamicForwardPort = port
				forwardConfig.SMBReverseDynamicForwardPath = common.GetFullPath(path)
			}

			run := &sshcmd.Run{
				Conf:                          data,
				ControlMasterOverride:         controlMasterOverride,
				PortForward:                   forwards,
				DynamicPortForward:            c.String("D"),
				HTTPDynamicPortForward:        c.String("d"),
				ReverseDynamicPortForward:     forwardConfig.ReverseDynamicPortForward,
				HTTPReverseDynamicPortForward: forwardConfig.HTTPReverseDynamicPortForward,
				NFSReverseDynamicForwardPort:  forwardConfig.NFSReverseDynamicForwardPort,
				NFSReverseDynamicForwardPath:  forwardConfig.NFSReverseDynamicForwardPath,
				SMBReverseDynamicForwardPort:  forwardConfig.SMBReverseDynamicForwardPort,
				SMBReverseDynamicForwardPath:  forwardConfig.SMBReverseDynamicForwardPath,
			}
			forwardConfig.ControlMasterOverride = controlMasterOverride
			if nfsForwarding := c.String("M"); nfsForwarding != "" {
				port, path, parseErr := common.ParseNFSForwardPortPath(nfsForwarding)
				if parseErr != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", parseErr)
					os.Exit(1)
				}
				run.NFSDynamicForwardPort = port
				run.NFSDynamicForwardPath = path
			}
			if smbForwarding := c.String("S"); smbForwarding != "" {
				port, path, parseErr := common.ParseNFSForwardPortPath(smbForwarding)
				if parseErr != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", parseErr)
					os.Exit(1)
				}
				run.SMBDynamicForwardPort = port
				run.SMBDynamicForwardPath = path
			}
			if enabled, local, remote, tunnelErr := resolveTunnelOption(runtime.GOOS, c.String("tunnel")); tunnelErr != nil {
				fmt.Fprintln(os.Stderr, tunnelErr)
				os.Exit(1)
			} else if enabled {
				run.TunnelEnabled = true
				run.TunnelLocal = local
				run.TunnelRemote = remote
			}
			forwardConfig.ParallelInfo = run.ParallelIgnoredFeatures

			if len(c.Args()) > 0 && runtime.GOOS != "windows" {
				stdin := 0
				if !terminal.IsTerminal(stdin) {
					stdinData, err = io.ReadAll(os.Stdin)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: %s\n", err)
						os.Exit(1)
					}
				}
			}
			if c.Bool("mux-child") {
				manager, err := mux.NewManager(data, names, c.Args(), stdinData, hosts, c.Bool("hold"), c.Bool("allow-layout-change"), forwardConfig)
				if err != nil {
					return err
				}
				return manager.Run()
			}
			if c.Bool("mux-daemon") {
				if muxSessionName == "" {
					muxSessionName = lsmuxsession.DefaultSessionName
				}
				exe, exeErr := os.Executable()
				if exeErr != nil {
					return exeErr
				}
				childArgs := make([]string, 0, len(os.Args))
				for _, arg := range os.Args[1:] {
					if arg == "--mux-daemon" {
						continue
					}
					childArgs = append(childArgs, arg)
				}
				childArgs = append(childArgs, "--mux-child")
				daemon := &lsmuxsession.Daemon{
					Name:       muxSessionName,
					ConfigPath: confpath,
					SocketPath: muxSocketPath,
					Exe:        exe,
					Args:       childArgs,
					Env:        append(os.Environ(), "_LSMUX_CHILD=1"),
				}
				return daemon.Run(notifyLsshMuxParentReady)
			}
			if muxSessionName != "" || c.Bool("mux-detach") {
				if runtime.GOOS == "windows" {
					return fmt.Errorf("persistent mux sessions are not supported on Windows yet")
				}
				if muxSessionName == "" {
					muxSessionName = lsmuxsession.DefaultSessionName
				}
				session, err := ensureLsshMuxSession(muxSessionName, muxSocketPath)
				if err != nil {
					return err
				}
				if c.Bool("mux-detach") {
					fmt.Fprintf(os.Stdout, "lsmux session %q is running in background (pid %d)\n", session.Name, session.PID)
					return nil
				}
				return lsmuxsession.Attach(session, lsmuxsession.AttachOptions{
					PrefixSpec: data.Mux.Prefix,
					DetachSpec: data.Mux.DetachClient,
				})
			}

			manager, err := mux.NewManager(data, names, c.Args(), stdinData, hosts, c.Bool("hold"), c.Bool("allow-layout-change"), forwardConfig)
			if err != nil {
				return err
			}
			return manager.Run()
		}

		selected := []string{}
		if len(hosts) > 0 {
			if !check.ExistServer(hosts, names) {
				fmt.Fprintln(os.Stderr, "Input Server not found from list.")
				os.Exit(1)
			} else {
				selected = hosts
			}
		} else {
			if len(names) == 0 {
				fmt.Fprintln(os.Stderr, "No servers matched the current config conditions.")
				os.Exit(1)
			}
			// View List And Get Select Line
			l := new(list.ListInfo)
			l.Prompt = "lssh>>"
			l.NameList = names
			l.DataList = data
			l.MultiFlag = isMulti

			l.View()
			selected = l.SelectName

			// Check selected
			if len(selected) == 0 {
				fmt.Fprintln(os.Stderr, "Selection cancelled.")
				os.Exit(1)
			}
			if selected[0] == "ServerName" {
				fmt.Fprintln(os.Stderr, "Server not selected.")
				os.Exit(1)
			}

			// If -f is specified, disallow selecting multiple hosts
			if c.Bool("f") && len(selected) > 1 {
				fmt.Fprintln(os.Stderr, "Error: -f cannot be used with multiple hosts. Select a single host.")
				os.Exit(1)
			}
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

		r := new(sshcmd.Run)
		r.ServerList = selected
		r.Conf = data
		r.ControlMasterOverride = controlMasterOverride
		switch {
		case c.Bool("pshell") == true && !c.Bool("not-execute"):
			r.Mode = "pshell"
		case len(c.Args()) > 0 && !c.Bool("not-execute"):
			// Becomes a shell when not-execute is given.
			r.Mode = "cmd"
		default:
			r.Mode = "shell"
		}

		// exec command
		r.ExecCmd = c.Args()
		r.ConnectorAttachSession = connectorAttachSession
		r.ConnectorDetach = connectorDetach
		r.IsParallel = c.Bool("parallel")

		if enableX11 || enableTrustedX11 {
			r.X11 = true
		}
		if enableTrustedX11 {
			r.X11Trusted = true
		}

		// is tty
		r.IsTerm = c.Bool("term")

		// local bashrc use
		r.IsBashrc = c.Bool("localrc")
		r.IsNotBashrc = c.Bool("not-localrc")

		// set w/W flag
		if c.Bool("w") {
			fmt.Println("enable w")
			r.EnableHeader = true
		}
		if c.Bool("W") {
			fmt.Println("enable W")
			r.DisableHeader = true
		}

		// Set port forwards
		var err error
		var forwards []*conf.PortForward

		// Set local port forwarding
		for _, forwardargs := range c.StringSlice("L") {
			f := new(conf.PortForward)
			f.Mode = "L"
			f.LocalNetwork, f.Local, f.RemoteNetwork, f.Remote, err = common.ParseForwardSpec(forwardargs)
			forwards = append(forwards, f)
		}

		// Set remote port forwarding
		for _, forwardargs := range c.StringSlice("R") {
			f := new(conf.PortForward)
			f.Mode = "R"

			// If only numbers are passed as arguments, treat as Reverse Dynamic Port Forward
			if regexp.MustCompile(`^[0-9]+$`).Match([]byte(forwardargs)) {
				r.ReverseDynamicPortForward = forwardargs
			} else {
				f.Local, f.Remote, err = common.ParseForwardPort(forwardargs)
				forwards = append(forwards, f)
			}
		}

		// Set NFS Forwarding
		nfsForwarding := c.String("M")
		if nfsForwarding != "" {
			port, path, err := common.ParseNFSForwardPortPath(nfsForwarding)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}

			r.NFSDynamicForwardPort = port
			r.NFSDynamicForwardPath = path
		}
		smbForwarding := c.String("S")
		if smbForwarding != "" {
			port, path, err := common.ParseNFSForwardPortPath(smbForwarding)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}

			r.SMBDynamicForwardPort = port
			r.SMBDynamicForwardPath = path
		}

		// Set NFS Reverse Forwarding
		nfsReverseForwarding := c.String("m")
		if nfsReverseForwarding != "" {
			port, path, err := common.ParseNFSForwardPortPath(nfsReverseForwarding)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}

			path = common.GetFullPath(path)

			r.NFSReverseDynamicForwardPort = port
			r.NFSReverseDynamicForwardPath = path
		}
		smbReverseForwarding := c.String("s")
		if smbReverseForwarding != "" {
			port, path, err := common.ParseNFSForwardPortPath(smbReverseForwarding)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}

			r.SMBReverseDynamicForwardPort = port
			r.SMBReverseDynamicForwardPath = common.GetFullPath(path)
		}

		// if err
		if err != nil {
			fmt.Printf("Error: %s \n", err)
		}

		// is not execute
		r.IsNone = c.Bool("not-execute")

		// Local/Remote port forwarding port
		r.PortForward = forwards

		// Dynamic port forwarding port
		r.DynamicPortForward = c.String("D")

		// HTTP Dynamic port forwarding port
		r.HTTPDynamicPortForward = c.String("d")

		// HTTP Reverse Dynamic port forwarding port
		r.HTTPReverseDynamicPortForward = c.String("r")

		// Tunnel device (like ssh -w local:remote). Format: <num|any>:<num|any>
		if enabled, local, remote, err := resolveTunnelOption(runtime.GOOS, c.String("tunnel")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		} else if enabled {
			r.TunnelEnabled = true
			r.TunnelLocal = local
			r.TunnelRemote = remote
		}

		// If -f specified and not already daemonized, re-exec to background
		if c.Bool("f") && os.Getenv("_LSSH_DAEMON") != "1" {
			// prepare args without -f
			args := []string{}
			for _, a := range os.Args[1:] {
				if a == "-f" || a == "--f" {
					continue
				}
				// also strip combined -fxxx is not supported; keep as-is
				args = append(args, a)
			}

			// If hosts were selected interactively, pass them to child via -H flags
			if len(hosts) == 0 {
				for _, s := range selected {
					args = append(args, "-H", s)
				}
			}

			exe, err := os.Executable()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}

			// create pipe for unix handshake
			var rpipe *os.File
			var wpipe *os.File
			if runtime.GOOS != "windows" {
				rpipe, wpipe, err = os.Pipe()
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error creating pipe:", err)
					os.Exit(1)
				}
			}

			cmd := exec.Command(exe, args...)
			// pass env to indicate daemonized child
			cmd.Env = append(os.Environ(), "_LSSH_DAEMON=1")

			if runtime.GOOS != "windows" {
				// pass write end as fd 3 in child
				cmd.ExtraFiles = []*os.File{wpipe}
				cmd.SysProcAttr = daemonSysProcAttr()
			}

			// detach stdin
			devnull, _ := os.OpenFile(os.DevNull, os.O_RDWR, 0)
			if devnull != nil {
				cmd.Stdin = devnull
			}

			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Start(); err != nil {
				fmt.Fprintln(os.Stderr, "Error starting background process:", err)
				os.Exit(1)
			}

			pid := 0
			if cmd.Process != nil {
				pid = cmd.Process.Pid
			}

			// close write end in parent
			if runtime.GOOS != "windows" && wpipe != nil {
				wpipe.Close()
			}

			// wait for child ready (unix only)
			if runtime.GOOS != "windows" && rpipe != nil {
				buf := make([]byte, 16)
				n, _ := rpipe.Read(buf)
				rpipe.Close()
				if n > 0 {
					fmt.Fprintf(os.Stderr, "Running in background (pid %d)\n", pid)
					os.Exit(0)
				}
				fmt.Fprintln(os.Stderr, "Background start failed")
				os.Exit(1)
			}

			// on windows just exit parent after starting child
			fmt.Fprintf(os.Stderr, "Running in background (pid %d)\n", pid)
			os.Exit(0)
		}

		r.Start()
		return nil
	}
	return app
}

func ensureLsshMuxSession(name, socketPath string) (lsmuxsession.Session, error) {
	if session, err := lsmuxsession.ResolveSession(name); err == nil {
		return session, nil
	}
	return spawnLsshMuxSession(name, socketPath)
}

func spawnLsshMuxSession(name, socketPath string) (lsmuxsession.Session, error) {
	args := make([]string, 0, len(os.Args)+6)
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--mux-detach", "--mux-attach", "--mux-list-sessions", "--mux-kill-session", "--mux-daemon", "--mux-child":
			continue
		}
		args = append(args, arg)
	}
	args = filterLsshMuxSessionValueFlags(args)
	args = append(args, "--mux-daemon", "--mux-session", name)
	if strings.TrimSpace(socketPath) != "" {
		args = append(args, "--mux-socket-path", socketPath)
	}

	exe, err := os.Executable()
	if err != nil {
		return lsmuxsession.Session{}, err
	}
	var rpipe *os.File
	var wpipe *os.File
	if runtime.GOOS != "windows" {
		rpipe, wpipe, err = os.Pipe()
		if err != nil {
			return lsmuxsession.Session{}, err
		}
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "_LSMUX_DAEMON=1")
	if runtime.GOOS != "windows" {
		cmd.ExtraFiles = []*os.File{wpipe}
		cmd.SysProcAttr = daemonSysProcAttr()
	}
	devnull, _ := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if devnull != nil {
		cmd.Stdin = devnull
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return lsmuxsession.Session{}, err
	}
	if runtime.GOOS != "windows" && wpipe != nil {
		_ = wpipe.Close()
	}
	if runtime.GOOS != "windows" && rpipe != nil {
		buf := make([]byte, 16)
		n, _ := rpipe.Read(buf)
		_ = rpipe.Close()
		if n == 0 {
			return lsmuxsession.Session{}, fmt.Errorf("background start failed")
		}
	}
	time.Sleep(200 * time.Millisecond)
	return lsmuxsession.ResolveSession(name)
}

func filterLsshMuxSessionValueFlags(args []string) []string {
	filtered := make([]string, 0, len(args))
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		switch arg {
		case "--mux-session", "--mux-socket-path":
			skipNext = true
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}

func notifyLsshMuxParentReady() {
	if os.Getenv("_LSMUX_DAEMON") != "1" {
		return
	}
	f := os.NewFile(uintptr(3), "lsmux_ready")
	if f == nil {
		return
	}
	defer f.Close()
	_, _ = f.Write([]byte("OK\n"))
}

func listLsshMuxSessions() error {
	sessions, err := lsmuxsession.ListSessions()
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		fmt.Fprintln(os.Stdout, "No lsmux sessions.")
		return nil
	}
	for i := range sessions {
		lsmuxsession.MarkSessionAlive(&sessions[i])
		fmt.Fprintln(os.Stdout, lsmuxsession.FormatSessionSummary(sessions[i]))
	}
	return nil
}

func killLsshMuxSession(name string) error {
	if strings.TrimSpace(name) == "" {
		name = lsmuxsession.DefaultSessionName
	}
	session, err := lsmuxsession.LoadSession(name)
	if err != nil {
		return err
	}
	if session.PID > 0 {
		process, findErr := os.FindProcess(session.PID)
		if findErr == nil {
			_ = process.Kill()
		}
	}
	return lsmuxsession.RemoveSession(name)
}

func attachLsshMuxSession(name string, cfg conf.Config) error {
	if strings.TrimSpace(name) == "" {
		name = lsmuxsession.DefaultSessionName
	}
	session, err := lsmuxsession.ResolveSession(name)
	if err != nil {
		return err
	}
	return lsmuxsession.Attach(session, lsmuxsession.AttachOptions{
		PrefixSpec: cfg.Mux.Prefix,
		DetachSpec: cfg.Mux.DetachClient,
	})
}
