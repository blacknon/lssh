package lssh

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
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
	cmd.Env = append(os.Environ(), "_LSSH_DAEMON=1")
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
		if n == 0 {
			return fmt.Errorf("background start failed")
		}
		fmt.Fprintf(os.Stderr, "Running in background (pid %d)\n", pid)
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "Running in background (pid %d)\n", pid)
	os.Exit(0)
	return nil
}
