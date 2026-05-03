// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsshfs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/core/apputil"
	mountfs "github.com/blacknon/lssh/internal/lsshfs"
	"github.com/blacknon/lssh/internal/version"
	"github.com/urfave/cli"
)

var (
	loadMountRecordFn   = mountfs.LoadMountRecord
	removeMountRecordFn = mountfs.RemoveMountRecord
	unmountCommandsFn   = mountfs.UnmountCommands
	normalizeMountPtFn  = mountfs.NormalizeMountPoint
	stateFilePathFn     = mountfs.StateFilePath
	execCommandFn       = exec.Command
	osExecutableFn      = os.Executable
)

const backgroundReadyTimeout = 15 * time.Second

func debugLogPath(mountpoint string) string {
	dir, err := mountfs.StateFilePath(mountpoint)
	if err != nil || strings.TrimSpace(dir) == "" {
		return ""
	}
	return strings.TrimSuffix(dir, filepath.Ext(dir)) + ".debug.log"
}

func Lsshfs() (app *cli.App) {
	defConf := common.GetDefaultConfigPath()

	cli.AppHelpTemplate = `NAME:
    {{.Name}} - {{.Usage}}
USAGE:
    {{.HelpName}} {{if .VisibleFlags}}[options]{{end}} [host:]remote_path mountpoint
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
    # mount a remote path from the selected host
    {{.Name}} /srv/data ~/mnt/data

    # mount a remote path from the named inventory host
    {{.Name}} @app:/srv/data ~/mnt/data

    # unmount
    {{.Name}} --unmount ~/mnt/data
`

	app = cli.NewApp()
	app.Name = "lsshfs"
	app.Usage = "Single-host SSH mount command with FUSE/NFS local mount backends."
	app.Copyright = "blacknon(blacknon@orebibou.com)"
	app.Version = version.AppVersion(app.Name)
	app.EnableBashCompletion = true
	app.HideHelp = true
	app.Flags = []cli.Flag{
		cli.StringSliceFlag{Name: "host,H", Usage: "connect `servername`."},
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config `filepath`."},
		cli.StringFlag{Name: "generate-lssh-conf", Usage: "print generated lssh config from OpenSSH config to stdout (`~/.ssh/config` by default)."},
		cli.StringSliceFlag{Name: "mount-option", Usage: "append local mount option (repeatable)."},
		cli.BoolFlag{Name: "debug", Usage: "enable debug logging for lsshfs and go-sshlib."},
		cli.BoolFlag{Name: "rw", Usage: "mount as read-write (current default behavior)."},
		cli.BoolFlag{Name: "unmount", Usage: "unmount the specified mountpoint and stop the background process."},
		cli.BoolFlag{Name: "list-mounts", Usage: "list active lsshfs mount records."},
		cli.BoolFlag{Name: "foreground", Usage: "run in the foreground for debugging and tests."},
		cli.BoolFlag{Name: "list,l", Usage: "print server list from config."},
		cli.BoolFlag{Name: "help,h", Usage: "print this help"},
	}
	app.Flags = append(app.Flags, common.ControlMasterOverrideFlags()...)

	app.Action = func(c *cli.Context) error {
		if c.Bool("debug") {
			_ = os.Setenv("GO_SSHLIB_DEBUG", "1")
			_ = os.Setenv("LSSHFS_DEBUG", "1")
		}

		if c.Bool("help") {
			cli.ShowAppHelp(c)
			return nil
		}

		data, handled, err := apputil.LoadConfigWithGenerateMode(c, os.Stdout, os.Stderr)
		if handled {
			return err
		}

		if c.Bool("list-mounts") {
			return printMountRecords()
		}

		if c.Bool("unmount") {
			if c.NArg() != 1 {
				return fmt.Errorf("--unmount requires exactly one mountpoint")
			}
			return unmountRecordedMount(c.Args()[0])
		}

		controlMasterOverride, controlMasterErr := common.GetControlMasterOverride(c)
		if controlMasterErr != nil {
			return controlMasterErr
		}

		if err != nil {
			return err
		}
		_, names, err := apputil.SortedServerNames(data, "")
		if err != nil {
			return err
		}

		if c.Bool("list") {
			apputil.PrintServerList(os.Stdout, names)
			return nil
		}

		if c.NArg() != 2 {
			cli.ShowAppHelp(c)
			return fmt.Errorf("lsshfs requires remote_path and mountpoint")
		}
		if c.Bool("debug") {
			if logPath := debugLogPath(c.Args()[1]); logPath != "" {
				_ = os.Setenv("LSSHFS_DEBUG_LOG", logPath)
				_ = os.Setenv("GO_SSHLIB_DEBUG_LOG", logPath)
				if c.Bool("foreground") && os.Getenv("_LSSHFS_DAEMON") != "1" {
					fmt.Fprintf(os.Stderr, "Debug log: %s\n", logPath)
				}
			}
		}

		specHost, remotePath, parseErr := mountfs.ParseRemoteSpec(c.Args()[0])
		if parseErr != nil {
			return parseErr
		}

		flagHosts := c.StringSlice("host")
		selectedHost, appendHostFlag, err := selectMountHost(flagHosts, names, data, specHost)
		if err != nil {
			return err
		}

		if !c.Bool("foreground") && os.Getenv("_LSSHFS_DAEMON") != "1" {
			return spawnBackgroundProcess(selectedHost, appendHostFlag)
		}

		runner := &mountfs.Runner{
			Config:                data,
			Host:                  selectedHost,
			RemotePath:            remotePath,
			MountPoint:            c.Args()[1],
			MountOptions:          lsshfsMountOptions(data, runtime.GOOS, c.StringSlice("mount-option")),
			ReadWrite:             c.Bool("rw") || !c.IsSet("rw"),
			GOOS:                  runtime.GOOS,
			ControlMasterOverride: controlMasterOverride,
			ReadyNotifier:         notifyParentReady,
			Stdout:                os.Stdout,
			Stderr:                os.Stderr,
		}

		return runner.Run()
	}

	return app
}

func lsshfsMountOptions(cfg conf.Config, goos string, cliOptions []string) []string {
	options := make([]string, 0)
	options = append(options, cfg.Lsshfs.MountOptions...)

	switch goos {
	case "darwin":
		options = append(options, cfg.Lsshfs.Darwin.MountOptions...)
	case "linux":
		options = append(options, cfg.Lsshfs.Linux.MountOptions...)
	case "windows":
		options = append(options, cfg.Lsshfs.Windows.MountOptions...)
	}

	options = append(options, cliOptions...)
	return mountfs.NormalizeMountOptions(options)
}

func spawnBackgroundProcess(selectedHost string, appendHostFlag bool) error {
	args := apputil.FilterCLIArgs(apputil.CurrentCLIArgs(), map[string]bool{"--foreground": true}, nil)

	if appendHostFlag {
		args = insertHostFlag(args, selectedHost)
	}

	pid, err := apputil.StartBackgroundProcess(apputil.BackgroundLaunchConfig{
		GOOS:             runtime.GOOS,
		Args:             args,
		DaemonEnvName:    "_LSSHFS_DAEMON",
		ReadyFileEnvName: "_LSSHFS_READY_FILE",
		ReadyTimeout:     backgroundReadyTimeout,
		ExtraEnv:         backgroundDebugEnv(args),
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
		Executable:       osExecutableFn,
		Command:          execCommandFn,
		Prepare: func(cmd *exec.Cmd) {
			if runtime.GOOS != "windows" {
				cmd.SysProcAttr = daemonSysProcAttr()
			}
		},
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Mounted in background (pid %d)\n", pid)
	if os.Getenv("LSSHFS_DEBUG") == "1" && len(args) > 0 {
		if logPath := debugLogPath(args[len(args)-1]); logPath != "" {
			fmt.Fprintf(os.Stderr, "Debug log: %s\n", logPath)
		}
	}
	os.Exit(0)
	return nil
}

func backgroundDebugEnv(args []string) []string {
	if os.Getenv("LSSHFS_DEBUG") != "1" || len(args) == 0 {
		return nil
	}

	logPath := debugLogPath(args[len(args)-1])
	if logPath == "" {
		return nil
	}

	return []string{
		"LSSHFS_DEBUG_LOG=" + logPath,
		"GO_SSHLIB_DEBUG_LOG=" + logPath,
	}
}

func insertHostFlag(args []string, host string) []string {
	insertAt := len(args)
	valueFlags := map[string]bool{
		"-F":                   true,
		"--file":               true,
		"-H":                   true,
		"--host":               true,
		"--generate-lssh-conf": true,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if valueFlags[arg] {
			i++
			continue
		}
		if len(arg) == 0 || arg[0] != '-' {
			insertAt = i
			break
		}
	}

	result := make([]string, 0, len(args)+2)
	result = append(result, args[:insertAt]...)
	result = append(result, "-H", host)
	result = append(result, args[insertAt:]...)
	return result
}

func notifyParentReady() {
	apputil.NotifyBackgroundReady("_LSSHFS_DAEMON", "_LSSHFS_READY_FILE")
}

func waitForBackgroundReadyFile(path string, timeout time.Duration) error {
	return apputil.WaitForBackgroundReadyFile(path, timeout)
}

func printMountRecords() error {
	records, err := mountfs.ListMountRecords()
	if err != nil {
		return err
	}

	if len(records) == 0 {
		fmt.Fprintln(os.Stdout, "No lsshfs mounts.")
		return nil
	}

	for _, record := range records {
		mode := "ro"
		if record.ReadWrite {
			mode = "rw"
		}
		fmt.Fprintf(os.Stdout, "%s\t%s:%s\t%s\tpid=%d\t%s\n", record.MountPoint, record.Host, record.RemotePath, record.Backend, record.PID, mode)
	}

	return nil
}

func unmountRecordedMount(mountpoint string) error {
	goos := runtime.GOOS
	normalizedMountPoint, err := normalizeMountPtFn(goos, mountpoint)
	if err != nil {
		return err
	}

	record, err := loadMountRecordFn(normalizedMountPoint)
	if err == nil && record.PID > 0 {
		process, findErr := os.FindProcess(record.PID)
		if findErr == nil {
			_ = process.Signal(syscall.SIGTERM)
		}

		for i := 0; i < 20; i++ {
			if _, statErr := os.Stat(processStatePath(normalizedMountPoint)); os.IsNotExist(statErr) {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	commands, err := unmountCommandsFn(goos, normalizedMountPoint)
	if err != nil {
		return err
	}
	for _, command := range commands {
		cmd := execCommandFn(command.Name, command.Args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if runErr := cmd.Run(); runErr == nil {
			break
		}
	}

	_ = removeMountRecordFn(normalizedMountPoint)
	return nil
}

func processStatePath(mountpoint string) string {
	path, err := stateFilePathFn(mountpoint)
	if err != nil {
		return ""
	}
	return path
}
