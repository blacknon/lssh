// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsmux

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/blacknon/lssh/internal/app/apputil"
	"github.com/blacknon/lssh/internal/check"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	lsmuxsession "github.com/blacknon/lssh/internal/lsmuxsession"
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

		data, handled, err := apputil.LoadConfigWithGenerateMode(c, os.Stdout, os.Stderr)
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
		socketPath := c.String("socket-path")
		if strings.TrimSpace(socketPath) == "" {
			socketPath = data.Mux.SocketPath
		}
		sessionName := c.String("session")
		if c.Bool("list-sessions") {
			return listMuxSessions()
		}
		if c.Bool("kill-session") {
			return killMuxSession(sessionName)
		}
		if c.Bool("attach") {
			return attachMuxSession(sessionName, data)
		}
		_, names, err := apputil.SortedServerNames(data, "")
		if err != nil {
			return err
		}

		if c.Bool("list") {
			apputil.PrintServerList(os.Stdout, names)
			return nil
		}

		initialHosts := c.StringSlice("host")
		if len(initialHosts) > 0 && !check.ExistServer(initialHosts, names) {
			return fmt.Errorf("input server not found from list")
		}
		forwardConfig := mux.SessionOptions{
			ControlMasterOverride: controlMasterOverride,
			IsBashrc:              c.Bool("localrc"),
			IsNotBashrc:           c.Bool("not-localrc"),
		}
		if c.Bool("enable-transfer") && c.Bool("disable-transfer") {
			return fmt.Errorf("--enable-transfer and --disable-transfer cannot be used together")
		}
		if c.Bool("enable-transfer") {
			enabled := true
			forwardConfig.TransferEnabled = &enabled
		}
		if c.Bool("disable-transfer") {
			enabled := false
			forwardConfig.TransferEnabled = &enabled
		}

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

		if c.Bool("mux-child") {
			manager, err := mux.NewManager(data, names, c.Args(), stdinData, initialHosts, c.Bool("hold"), c.Bool("allow-layout-change"), forwardConfig)
			if err != nil {
				return err
			}
			return manager.Run()
		}
		if c.Bool("mux-daemon") {
			if sessionName == "" {
				sessionName = lsmuxsession.DefaultSessionName
			}
			exe, exeErr := os.Executable()
			if exeErr != nil {
				return exeErr
			}
			childArgs := apputil.FilterCLIArgs(apputil.CurrentCLIArgs(), map[string]bool{"--mux-daemon": true}, nil)
			childArgs = append(childArgs, "--mux-child")
			daemon := &lsmuxsession.Daemon{
				Name:       sessionName,
				ConfigPath: c.String("file"),
				SocketPath: socketPath,
				Exe:        exe,
				Args:       childArgs,
				Env:        append(os.Environ(), "_LSMUX_CHILD=1"),
			}
			return daemon.Run(notifyMuxParentReady)
		}

		if sessionName != "" || c.Bool("detach") {
			if runtime.GOOS == "windows" {
				return fmt.Errorf("persistent lsmux sessions are not supported on Windows yet")
			}
			if sessionName == "" {
				sessionName = lsmuxsession.DefaultSessionName
			}
			session, err := ensureMuxSession(sessionName, socketPath)
			if err != nil {
				return err
			}
			if c.Bool("detach") {
				fmt.Fprintf(os.Stdout, "lsmux session %q is running in background (pid %d)\n", session.Name, session.PID)
				return nil
			}
			return lsmuxsession.Attach(session, lsmuxsession.AttachOptions{
				PrefixSpec: data.Mux.Prefix,
				DetachSpec: data.Mux.DetachClient,
			})
		}

		manager, err := mux.NewManager(data, names, c.Args(), stdinData, initialHosts, c.Bool("hold"), c.Bool("allow-layout-change"), forwardConfig)
		if err != nil {
			return err
		}
		return manager.Run()
	}

	return app
}

func listMuxSessions() error {
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

func killMuxSession(name string) error {
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

func attachMuxSession(name string, cfg conf.Config) error {
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

func ensureMuxSession(name, socketPath string) (lsmuxsession.Session, error) {
	if session, err := lsmuxsession.ResolveSession(name); err == nil {
		return session, nil
	}
	return spawnMuxSession(name, socketPath)
}

func spawnMuxSession(name, socketPath string) (lsmuxsession.Session, error) {
	args := apputil.FilterCLIArgs(apputil.CurrentCLIArgs(), map[string]bool{
		"--detach":        true,
		"--attach":        true,
		"--list-sessions": true,
		"--kill-session":  true,
		"--mux-daemon":    true,
		"--mux-child":     true,
	}, map[string]bool{
		"--session":     true,
		"--socket-path": true,
	})
	args = append(args, "--mux-daemon", "--session", name)
	if strings.TrimSpace(socketPath) != "" {
		args = append(args, "--socket-path", socketPath)
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

func filterMuxSessionValueFlags(args []string) []string {
	return apputil.FilterCLIArgs(args, nil, map[string]bool{
		"--session":     true,
		"--socket-path": true,
	})
}

func notifyMuxParentReady() {
	apputil.NotifyBackgroundReady("_LSMUX_DAEMON", "")
}
