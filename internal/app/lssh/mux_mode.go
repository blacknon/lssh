package lssh

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

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
	args := make([]string, 0, len(allArgs)+4)
	for _, arg := range allArgs {
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

	return args
}

func runMuxDaemon(confpath, muxSessionName, muxSocketPath string) error {
	if muxSessionName == "" {
		muxSessionName = lsmuxsession.DefaultSessionName
	}
	exe, err := os.Executable()
	if err != nil {
		return err
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
	args := buildMuxDaemonArgs(os.Args[1:], name, socketPath)

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
