// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsshfs

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"syscall"
	"time"

	"github.com/blacknon/lssh/internal/check"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/list"
	mountfs "github.com/blacknon/lssh/internal/lsshfs"
	"github.com/blacknon/lssh/internal/version"
	"github.com/urfave/cli"
)

var (
	loadMountRecordFn    = mountfs.LoadMountRecord
	removeMountRecordFn  = mountfs.RemoveMountRecord
	unmountCommandsFn    = mountfs.UnmountCommands
	normalizeMountPtFn   = mountfs.NormalizeMountPoint
	stateFilePathFn      = mountfs.StateFilePath
	execCommandFn        = exec.Command
)

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

    # windows example
    {{.Name}} @app:/srv/data Z:
`

	app = cli.NewApp()
	app.Name = "lsshfs"
	app.Usage = "Single-host SSH mount command with OS-specific local mount backends."
	app.Copyright = "blacknon(blacknon@orebibou.com)"
	app.Version = version.AppVersion(app.Name)
	app.EnableBashCompletion = true
	app.HideHelp = true
	app.Flags = []cli.Flag{
		cli.StringSliceFlag{Name: "host,H", Usage: "connect `servername`."},
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config `filepath`."},
		cli.StringFlag{Name: "generate-lssh-conf", Usage: "print generated lssh config from OpenSSH config to stdout (`~/.ssh/config` by default)."},
		cli.BoolFlag{Name: "rw", Usage: "mount as read-write (current default behavior)."},
		cli.BoolFlag{Name: "unmount", Usage: "unmount the specified mountpoint and stop the background process."},
		cli.BoolFlag{Name: "list-mounts", Usage: "list active lsshfs mount records."},
		cli.BoolFlag{Name: "foreground", Usage: "run in the foreground for debugging and tests."},
		cli.BoolFlag{Name: "list,l", Usage: "print server list from config."},
		cli.BoolFlag{Name: "help,h", Usage: "print this help"},
	}
	app.Flags = append(app.Flags, common.ControlMasterOverrideFlags()...)

	app.Action = func(c *cli.Context) error {
		if c.Bool("help") {
			cli.ShowAppHelp(c)
			os.Exit(0)
		}

		if handled, err := conf.HandleGenerateConfigMode(c.String("generate-lssh-conf"), os.Stdout); handled {
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

		if c.NArg() != 2 {
			cli.ShowAppHelp(c)
			return fmt.Errorf("lsshfs requires remote_path and mountpoint")
		}

		specHost, remotePath, parseErr := mountfs.ParseRemoteSpec(c.Args()[0])
		if parseErr != nil {
			return parseErr
		}

		flagHosts := c.StringSlice("host")
		if len(flagHosts) > 1 {
			return fmt.Errorf("lsshfs only supports a single host")
		}
		if len(flagHosts) > 0 && !check.ExistServer(flagHosts, names) {
			return fmt.Errorf("input server not found from list")
		}
		if specHost != "" && !check.ExistServer([]string{specHost}, names) {
			return fmt.Errorf("input server not found from list")
		}

		selectedHost := ""
		switch {
		case specHost != "" && len(flagHosts) == 1 && specHost != flagHosts[0]:
			return fmt.Errorf("host in remote path and --host do not match")
		case specHost != "":
			selectedHost = specHost
		case len(flagHosts) == 1:
			selectedHost = flagHosts[0]
		default:
			l := new(list.ListInfo)
			l.Prompt = "lsshfs>>"
			l.NameList = names
			l.DataList = data
			l.MultiFlag = false
			l.View()
			if len(l.SelectName) == 0 || l.SelectName[0] == "ServerName" {
				return fmt.Errorf("server not selected")
			}
			selectedHost = l.SelectName[0]
		}

		if !c.Bool("foreground") && os.Getenv("_LSSHFS_DAEMON") != "1" {
			return spawnBackgroundProcess(selectedHost, specHost == "" && len(flagHosts) == 0)
		}

		runner := &mountfs.Runner{
			Config:                data,
			Host:                  selectedHost,
			RemotePath:            remotePath,
			MountPoint:            c.Args()[1],
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

func spawnBackgroundProcess(selectedHost string, appendHostFlag bool) error {
	args := make([]string, 0, len(os.Args))
	for _, arg := range os.Args[1:] {
		if arg == "--foreground" {
			continue
		}
		args = append(args, arg)
	}

	if appendHostFlag {
		args = insertHostFlag(args, selectedHost)
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	var rpipe *os.File
	var wpipe *os.File
	if runtime.GOOS != "windows" {
		rpipe, wpipe, err = os.Pipe()
		if err != nil {
			return err
		}
	}

	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "_LSSHFS_DAEMON=1")
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
		return err
	}

	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}

	if runtime.GOOS != "windows" && wpipe != nil {
		_ = wpipe.Close()
	}
	if runtime.GOOS != "windows" && rpipe != nil {
		buf := make([]byte, 16)
		n, _ := rpipe.Read(buf)
		_ = rpipe.Close()
		if n > 0 {
			fmt.Fprintf(os.Stderr, "Mounted in background (pid %d)\n", pid)
			os.Exit(0)
		}
		return fmt.Errorf("background start failed")
	}

	fmt.Fprintf(os.Stderr, "Mounted in background (pid %d)\n", pid)
	os.Exit(0)
	return nil
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
	if os.Getenv("_LSSHFS_DAEMON") != "1" {
		return
	}

	f := os.NewFile(uintptr(3), "lsshfs_ready")
	if f == nil {
		return
	}
	defer f.Close()

	_, _ = f.Write([]byte("OK\n"))
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
