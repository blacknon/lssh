package lssh

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/blacknon/lssh/internal/app/apputil"
	conf "github.com/blacknon/lssh/internal/config"
	lsmuxsession "github.com/blacknon/lssh/internal/lsmuxsession"
	"github.com/blacknon/lssh/internal/mux"
	"github.com/urfave/cli"
)

func runMuxMode(c *cli.Context, data conf.Config, names, hosts []string, confpath string, controlMasterOverride *bool, enableX11, enableTrustedX11 bool, connectorAttachSession string, connectorDetach bool, muxSessionName, muxSocketPath string) error {
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
	if err := validateKnownHosts(hosts, names); err != nil {
		return err
	}

	forwardConfig, err := buildMuxSessionOptions(c, data, controlMasterOverride, enableX11, enableTrustedX11)
	if err != nil {
		return err
	}
	stdinData, err := readMuxStdinData(c)
	if err != nil {
		return err
	}

	if c.Bool("mux-child") {
		return runMuxChild(data, names, c.Args(), stdinData, hosts, c.Bool("hold"), c.Bool("allow-layout-change"), forwardConfig)
	}
	if c.Bool("mux-daemon") {
		return runMuxDaemon(confpath, muxSessionName, muxSocketPath)
	}
	if muxSessionName != "" || c.Bool("mux-detach") {
		return runPersistentMuxSession(c, data, muxSessionName, muxSocketPath)
	}

	return runMuxChild(data, names, c.Args(), stdinData, hosts, c.Bool("hold"), c.Bool("allow-layout-change"), forwardConfig)
}

func runMuxChild(data conf.Config, names, command []string, stdinData []byte, hosts []string, hold, allowLayoutChange bool, forwardConfig mux.SessionOptions) error {
	manager, err := mux.NewManager(data, names, command, stdinData, hosts, hold, allowLayoutChange, forwardConfig)
	if err != nil {
		return err
	}
	return manager.Run()
}

func buildMuxDaemonArgs(allArgs []string, name, socketPath string) []string {
	return apputil.BuildPersistentSessionArgs(apputil.PersistentSessionArgsConfig{
		AllArgs: allArgs,
		BareFlags: map[string]bool{
			"--mux-detach":        true,
			"--mux-attach":        true,
			"--mux-list-sessions": true,
			"--mux-kill-session":  true,
			"--mux-daemon":        true,
			"--mux-child":         true,
		},
		ValueFlags: map[string]bool{
			"--mux-session":     true,
			"--mux-socket-path": true,
		},
		DaemonFlag:  "--mux-daemon",
		SessionFlag: "--mux-session",
		SocketFlag:  "--mux-socket-path",
		Name:        name,
		SocketPath:  socketPath,
	})
}

func runMuxDaemon(confpath, muxSessionName, muxSocketPath string) error {
	if muxSessionName == "" {
		muxSessionName = lsmuxsession.DefaultSessionName
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	childArgs := apputil.FilterCLIArgs(apputil.CurrentCLIArgs(), map[string]bool{"--mux-daemon": true}, nil)
	childArgs = append(childArgs, "--mux-child")
	return apputil.RunMuxSessionDaemon(apputil.MuxSessionDaemonConfig{
		Name:       muxSessionName,
		ConfigPath: confpath,
		SocketPath: muxSocketPath,
		Exe:        exe,
		Args:       childArgs,
		Env:        append(os.Environ(), "_LSMUX_CHILD=1"),
		Ready:      notifyLsshMuxParentReady,
	})
}

func runPersistentMuxSession(c *cli.Context, data conf.Config, muxSessionName, muxSocketPath string) error {
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

func ensureLsshMuxSession(name, socketPath string) (lsmuxsession.Session, error) {
	if session, err := lsmuxsession.ResolveSession(name); err == nil {
		return session, nil
	}
	return spawnLsshMuxSession(name, socketPath)
}

func spawnLsshMuxSession(name, socketPath string) (lsmuxsession.Session, error) {
	args := buildMuxDaemonArgs(apputil.CurrentCLIArgs(), name, socketPath)
	return apputil.SpawnMuxSession(apputil.MuxSessionSpawnConfig{
		GOOS:          runtime.GOOS,
		Name:          name,
		DaemonEnvName: "_LSMUX_DAEMON",
		Args:          args,
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
		Prepare: func(cmd *exec.Cmd) {
			if runtime.GOOS != "windows" {
				cmd.SysProcAttr = daemonSysProcAttr()
			}
		},
		Resolve: lsmuxsession.ResolveSession,
	})
}

func filterLsshMuxSessionValueFlags(args []string) []string {
	return apputil.FilterCLIArgs(args, nil, map[string]bool{
		"--mux-session":     true,
		"--mux-socket-path": true,
	})
}

func notifyLsshMuxParentReady() {
	apputil.NotifyBackgroundReady("_LSMUX_DAEMON", "")
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
