package lssh

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/blacknon/lssh/internal/app/apputil"
)

func buildBackgroundArgs(allArgs, selectedHosts, explicitHosts []string) []string {
	args := make([]string, 0, len(allArgs)+len(selectedHosts)*2)
	for _, arg := range allArgs {
		if arg == "-f" || arg == "--f" {
			continue
		}
		args = append(args, arg)
	}

	if len(explicitHosts) == 0 {
		for _, host := range selectedHosts {
			args = append(args, "-H", host)
		}
	}

	return args
}

func runBackgroundLssh(selectedHosts, explicitHosts []string) error {
	args := buildBackgroundArgs(os.Args[1:], selectedHosts, explicitHosts)
	pid, err := apputil.StartBackgroundProcess(apputil.BackgroundLaunchConfig{
		GOOS:          runtime.GOOS,
		Args:          args,
		DaemonEnvName: "_LSSH_DAEMON",
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
		Prepare: func(cmd *exec.Cmd) {
			if runtime.GOOS != "windows" {
				cmd.SysProcAttr = daemonSysProcAttr()
			}
		},
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Running in background (pid %d)\n", pid)
	os.Exit(0)
	return nil
}
