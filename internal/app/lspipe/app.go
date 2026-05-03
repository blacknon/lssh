// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lspipe

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/blacknon/lssh/internal/app/apputil"
	"github.com/blacknon/lssh/internal/check"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/list"
	pipeapp "github.com/blacknon/lssh/internal/lspipe"
	"github.com/blacknon/lssh/internal/version"
	"github.com/urfave/cli"
)

var (
	loadSessionFn        = pipeapp.LoadSession
	markSessionAliveFn   = pipeapp.MarkSessionAlive
	formatSessionFn      = pipeapp.FormatSessionSummary
	resolveSessionFn     = pipeapp.ResolveSession
	listFIFORecordsFn    = pipeapp.ListFIFORecords
	removeSessionFn      = pipeapp.RemoveSession
	pingSessionFn        = pipeapp.PingSession
	buildFIFOEndpointsFn = pipeapp.BuildFIFOEndpoints
	loadFIFORecordFn     = pipeapp.LoadFIFORecord
	saveFIFORecordFn     = pipeapp.SaveFIFORecord
	removeFIFORecordFn   = pipeapp.RemoveFIFORecord
	executePipeFn        = pipeapp.Execute
	spawnDaemonFn        = spawnDaemon
)

func Lspipe() (app *cli.App) {
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
    # create default session from TUI
    {{.Name}}

    # create named session from cli
    {{.Name}} --name prod -H web01 -H web02

    # execute command through existing session
    {{.Name}} hostname
    echo test | {{.Name}} 'cat'

    # single host raw output
    {{.Name}} -H web01 --raw cat /etc/hosts

    # select session hosts by the index shown in --info
    {{.Name}} -H 2 --raw cat /etc/hosts
`

	app = cli.NewApp()
	app.Name = "lspipe"
	app.Usage = "Persistent SSH pipe sessions for reusing selected hosts from local shell pipelines."
	app.Copyright = "blacknon(blacknon@orebibou.com)"
	app.Version = version.AppVersion(app.Name)
	app.EnableBashCompletion = true
	app.HideHelp = true
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "name", Value: pipeapp.DefaultSessionName, Usage: "session `name`."},
		cli.StringFlag{Name: "fifo-name", Value: "default", Usage: "named pipe set `name`."},
		cli.StringSliceFlag{Name: "host,H", Usage: "add session `servername` on creation, or limit execution to session `servername` or `index` shown by --info."},
		cli.StringSliceFlag{Name: "create-host", Hidden: true},
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config `filepath`."},
		cli.StringFlag{Name: "generate-lssh-conf", Usage: "print generated lssh config from OpenSSH config to stdout (`~/.ssh/config` by default)."},
		cli.BoolFlag{Name: "replace", Usage: "replace the named session if it already exists."},
		cli.BoolFlag{Name: "list", Usage: "list known lspipe sessions."},
		cli.BoolFlag{Name: "mkfifo", Usage: "create a named pipe bridge for the named session."},
		cli.BoolFlag{Name: "list-fifos", Usage: "list named pipe bridges."},
		cli.BoolFlag{Name: "rmfifo", Usage: "remove the named pipe bridge for the named session."},
		cli.BoolFlag{Name: "info", Usage: "show information for the named session."},
		cli.BoolFlag{Name: "close", Usage: "close the named session."},
		cli.BoolFlag{Name: "raw", Usage: "write pure stdout for exactly one resolved host."},
		cli.BoolFlag{Name: "daemon", Hidden: true},
		cli.BoolFlag{Name: "fifo-worker", Hidden: true},
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

		name := strings.TrimSpace(c.String("name"))
		if name == "" {
			name = pipeapp.DefaultSessionName
		}

		if c.Bool("daemon") {
			return runDaemon(c, config, name, controlMasterOverride)
		}
		if c.Bool("fifo-worker") {
			return runFIFOWorker(c, name)
		}

		if c.Bool("list") {
			return listSessions()
		}
		if c.Bool("list-fifos") {
			return listFIFOs()
		}
		if c.Bool("info") {
			return printSessionInfo(name)
		}
		if c.Bool("mkfifo") {
			return ensureFIFOBridge(c, name)
		}
		if c.Bool("rmfifo") {
			return removeFIFOBridge(name, c.String("fifo-name"))
		}
		if c.Bool("close") {
			return closeSession(name)
		}

		command := strings.TrimSpace(strings.Join(c.Args(), " "))
		if command == "" {
			return ensureSession(c, config, name)
		}

		if err := ensureSessionForCommand(c, config, name); err != nil {
			return err
		}

		stdinData, err := readPipeInput(os.Stdin)
		if err != nil {
			return err
		}

		return executePipeFn(pipeapp.ExecOptions{
			Name:    name,
			Command: command,
			Hosts:   selectedHosts(c),
			Raw:     c.Bool("raw"),
			Stdin:   stdinData,
			Stdout:  os.Stdout,
			Stderr:  os.Stderr,
		})
	}

	return app
}

func ensureSession(c *cli.Context, config conf.Config, name string) error {
	return ensureSessionState(c, config, name, true)
}

func ensureSessionForCommand(c *cli.Context, config conf.Config, name string) error {
	return ensureSessionState(c, config, name, false)
}

func ensureSessionState(c *cli.Context, config conf.Config, name string, printReuse bool) error {
	current, err := loadSessionFn(name)
	if err == nil {
		markSessionAliveFn(&current)
		if !current.Stale && !c.Bool("replace") {
			if printReuse {
				fmt.Fprintln(os.Stdout, formatSessionFn(current))
			}
			return nil
		}
		_ = closeSession(name)
	}

	selectedHosts, err := resolveCreateHosts(c, config)
	if err != nil {
		return err
	}

	return spawnDaemonFn(c, name, selectedHosts)
}

func resolveCreateHosts(c *cli.Context, config conf.Config) ([]string, error) {
	names := conf.GetNameList(config)
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("no servers matched the current config conditions")
	}

	hosts := selectedHosts(c)
	if len(hosts) > 0 {
		if !check.ExistServer(hosts, names) {
			return nil, fmt.Errorf("input server not found from list")
		}
		sort.Strings(hosts)
		return hosts, nil
	}

	selected, ok, err := list.SelectHosts("lspipe>>", names, config, true)
	if err != nil {
		return nil, err
	}
	if !ok || len(selected) == 0 {
		return nil, fmt.Errorf("selection cancelled")
	}
	if !check.ExistServer(selected, names) {
		return nil, fmt.Errorf("input server not found from list")
	}
	sort.Strings(selected)
	return selected, nil
}

func spawnDaemon(c *cli.Context, name string, hosts []string) error {
	args := make([]string, 0, len(os.Args)+4+len(hosts)*2)
	args = append(args, apputil.CurrentCLIArgs()...)
	args = filterNonDaemonArgs(args)
	args = append(args, "--daemon", "--name", name)
	for _, host := range hosts {
		args = append(args, "-H", host)
	}

	pid, err := startBackgroundLspipeProcess(args)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "lspipe session %q is ready in background (pid %d)\n", name, pid)
	return nil
}

func runDaemon(c *cli.Context, config conf.Config, name string, controlMasterOverride *bool) error {
	hosts := selectedHosts(c)
	if len(hosts) == 0 {
		return fmt.Errorf("daemon mode requires at least one host")
	}

	daemon := pipeapp.NewDaemon(name, c.String("file"), config, hosts, controlMasterOverride)
	return daemon.Run(notifyParentReady)
}

func filterNonDaemonArgs(args []string) []string {
	filtered := make([]string, 0, len(args))
	filteredValueFlags := map[string]bool{
		"--name":               true,
		"--fifo-name":          true,
		"--host":               true,
		"--create-host":        true,
		"--generate-lssh-conf": true,
		"-H":                   true,
	}
	preservedValueFlags := map[string]bool{
		"--file":                   true,
		"-F":                       true,
		"--enable-control-master":  true,
		"--disable-control-master": true,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--replace", "--daemon", "--fifo-worker", "--list", "--list-fifos", "--mkfifo", "--rmfifo", "--info", "--close", "--raw":
			continue
		}
		if filteredValueFlags[arg] {
			i++
			continue
		}
		if preservedValueFlags[arg] {
			filtered = append(filtered, arg)
			if i+1 < len(args) {
				i++
				filtered = append(filtered, args[i])
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			filtered = append(filtered, arg)
			continue
		}
		break
	}
	return filtered
}

func selectedHosts(c *cli.Context) []string {
	hosts := append([]string{}, c.StringSlice("host")...)
	hosts = append(hosts, c.StringSlice("create-host")...)
	return hosts
}

func notifyParentReady() {
	apputil.NotifyBackgroundReady("_LSPipe_DAEMON", "")
}

func listSessions() error {
	sessions, err := pipeapp.ListSessions()
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		fmt.Fprintln(os.Stdout, "No lspipe sessions.")
		return nil
	}

	for i := range sessions {
		pipeapp.MarkSessionAlive(&sessions[i])
		fmt.Fprintln(os.Stdout, pipeapp.FormatSessionSummary(sessions[i]))
	}
	return nil
}

func printSessionInfo(name string) error {
	session, err := loadSessionFn(name)
	if err != nil {
		return err
	}
	pipeapp.MarkSessionAlive(&session)

	fmt.Fprintf(os.Stdout, "name: %s\n", session.Name)
	fmt.Fprintf(os.Stdout, "status: %s\n", ternaryStatus(session.Stale))
	fmt.Fprintf(os.Stdout, "pid: %d\n", session.PID)
	fmt.Fprintf(os.Stdout, "network: %s\n", session.Network)
	fmt.Fprintf(os.Stdout, "address: %s\n", session.Address)
	fmt.Fprintf(os.Stdout, "created: %s\n", session.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(os.Stdout, "last used: %s\n", session.LastUsedAt.Format(time.RFC3339))
	fmt.Fprintf(os.Stdout, "hosts:\n")
	for i, host := range session.Hosts {
		health := session.HostHealth[host]
		status := "connected"
		if !health.Connected && health.Error != "" {
			status = "error: " + health.Error
		} else if !health.Connected {
			status = "unknown"
		}
		fmt.Fprintf(os.Stdout, "  - [%d] %s (%s)\n", i+1, host, status)
	}
	return nil
}

func closeSession(name string) error {
	session, err := pipeapp.LoadSession(name)
	if err != nil {
		return err
	}

	records, _ := listFIFORecordsFn()
	for _, record := range records {
		if record.SessionName == session.Name {
			_ = removeFIFOBridge(record.SessionName, record.Name)
		}
	}

	if session.PID > 0 {
		process, findErr := os.FindProcess(session.PID)
		if findErr == nil {
			if runtime.GOOS == "windows" {
				_ = process.Kill()
			} else {
				_ = process.Signal(syscall.SIGTERM)
			}
		}
	}

	for i := 0; i < 20; i++ {
		if !pingSessionFn(session) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return removeSessionFn(name)
}

func readPipeInput(r io.Reader) ([]byte, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return nil, nil
	}
	return io.ReadAll(r)
}

func ternaryStatus(stale bool) string {
	if stale {
		return "stale"
	}
	return "alive"
}

func ensureFIFOBridge(c *cli.Context, sessionName string) error {
	session, err := resolveSessionFn(sessionName)
	if err != nil {
		return err
	}

	fifoName := c.String("fifo-name")
	record, err := loadFIFORecordFn(session.Name, fifoName)
	if err == nil {
		fmt.Fprintf(os.Stdout, "%s\t%s\tpid=%d\n", record.SessionName, record.Dir, record.PID)
		return nil
	}

	endpoints, baseDir, err := buildFIFOEndpointsFn(session, fifoName)
	if err != nil {
		return err
	}

	if err := spawnFIFOWorker(c, session.Name, fifoName, session.Hosts); err != nil {
		return err
	}

	record, err = loadFIFORecordFn(session.Name, fifoName)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "lspipe fifo %q for session %q is ready in background (pid %d)\n", fifoName, session.Name, record.PID)
	for _, endpoint := range endpoints {
		fmt.Fprintf(os.Stdout, "%s\n", endpoint.CmdPath)
		fmt.Fprintf(os.Stdout, "%s\n", endpoint.StdinPath)
		fmt.Fprintf(os.Stdout, "%s\n", endpoint.OutPath)
	}
	fmt.Fprintf(os.Stdout, "base: %s\n", baseDir)
	return nil
}

func spawnFIFOWorker(c *cli.Context, sessionName, fifoName string, hosts []string) error {
	args := make([]string, 0, len(os.Args)+8+len(hosts)*2)
	args = append(args, apputil.CurrentCLIArgs()...)
	args = filterNonDaemonArgs(args)
	args = append(args, "--fifo-worker", "--name", sessionName, "--fifo-name", fifoName)
	for _, host := range hosts {
		args = append(args, "-H", host)
	}

	pid, err := startBackgroundLspipeProcess(args)
	if err != nil {
		return fmt.Errorf("background fifo start failed: %w", err)
	}

	record, err := loadFIFORecordFn(sessionName, fifoName)
	if err == nil {
		record.PID = pid
		_ = saveFIFORecordFn(record)
	}

	return nil
}

func startBackgroundLspipeProcess(args []string) (int, error) {
	return apputil.StartBackgroundProcess(apputil.BackgroundLaunchConfig{
		GOOS:          runtime.GOOS,
		Args:          args,
		DaemonEnvName: "_LSPipe_DAEMON",
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
		Prepare: func(cmd *exec.Cmd) {
			if runtime.GOOS != "windows" {
				cmd.SysProcAttr = daemonSysProcAttr()
			}
		},
	})
}

func runFIFOWorker(c *cli.Context, sessionName string) error {
	session, err := resolveSessionFn(sessionName)
	if err != nil {
		return err
	}

	fifoName := c.String("fifo-name")
	endpoints, baseDir, err := buildFIFOEndpointsFn(session, fifoName)
	if err != nil {
		return err
	}

	if err := saveFIFORecordFn(pipeapp.FIFORecord{
		SessionName: session.Name,
		Name:        fifoName,
		Dir:         baseDir,
		PID:         os.Getpid(),
		Hosts:       session.Hosts,
	}); err != nil {
		return err
	}

	notifyParentReady()
	worker := &pipeapp.FIFOWorker{
		SessionName: session.Name,
		FIFOName:    fifoName,
		Endpoints:   endpoints,
	}
	return worker.Run()
}

func listFIFOs() error {
	records, err := listFIFORecordsFn()
	if err != nil {
		return err
	}
	if len(records) == 0 {
		fmt.Fprintln(os.Stdout, "No lspipe fifo bridges.")
		return nil
	}

	for _, record := range records {
		fmt.Fprintf(os.Stdout, "%s\t%s\tpid=%d\thosts=%d\t%s\n", record.SessionName, record.Name, record.PID, len(record.Hosts), record.Dir)
	}
	return nil
}

func removeFIFOBridge(sessionName, fifoName string) error {
	record, err := loadFIFORecordFn(sessionName, fifoName)
	if err != nil {
		return err
	}

	if record.PID > 0 {
		process, findErr := os.FindProcess(record.PID)
		if findErr == nil {
			if runtime.GOOS == "windows" {
				_ = process.Kill()
			} else {
				_ = process.Signal(syscall.SIGTERM)
			}
		}
	}

	_ = os.RemoveAll(record.Dir)
	return removeFIFORecordFn(sessionName, fifoName)
}
