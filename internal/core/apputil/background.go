package apputil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type BackgroundLaunchConfig struct {
	GOOS             string
	Args             []string
	DaemonEnvName    string
	ReadyFileEnvName string
	ReadyTimeout     time.Duration
	ExtraEnv         []string
	Stdout           *os.File
	Stderr           *os.File
	Executable       func() (string, error)
	Command          func(name string, arg ...string) *exec.Cmd
	Prepare          func(cmd *exec.Cmd)
}

func StartBackgroundProcess(cfg BackgroundLaunchConfig) (int, error) {
	executable := cfg.Executable
	if executable == nil {
		executable = os.Executable
	}

	command := cfg.Command
	if command == nil {
		command = exec.Command
	}

	exe, err := executable()
	if err != nil {
		return 0, err
	}

	var rpipe *os.File
	var wpipe *os.File
	if cfg.GOOS != "windows" {
		rpipe, wpipe, err = os.Pipe()
		if err != nil {
			return 0, err
		}
	}

	var readyDir string
	var readyPath string
	if cfg.GOOS == "windows" && strings.TrimSpace(cfg.ReadyFileEnvName) != "" {
		readyDir, err = os.MkdirTemp("", "lssh-ready-*")
		if err != nil {
			return 0, err
		}
		defer os.RemoveAll(readyDir)
		readyPath = filepath.Join(readyDir, "ready")
	}

	cmd := command(exe, cfg.Args...)
	cmd.Env = append(os.Environ(), cfg.DaemonEnvName+"=1")
	cmd.Env = append(cmd.Env, cfg.ExtraEnv...)
	if readyPath != "" {
		cmd.Env = append(cmd.Env, cfg.ReadyFileEnvName+"="+readyPath)
	}

	if cfg.GOOS != "windows" {
		cmd.ExtraFiles = []*os.File{wpipe}
	}
	if cfg.Prepare != nil {
		cfg.Prepare(cmd)
	}

	devnull, _ := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if devnull != nil {
		cmd.Stdin = devnull
	}
	cmd.Stdout = cfg.Stdout
	cmd.Stderr = cfg.Stderr

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}

	if cfg.GOOS != "windows" && wpipe != nil {
		_ = wpipe.Close()
	}
	if cfg.GOOS != "windows" && rpipe != nil {
		buf := make([]byte, 16)
		n, _ := rpipe.Read(buf)
		_ = rpipe.Close()
		if n == 0 {
			return 0, fmt.Errorf("background start failed")
		}
		return pid, nil
	}

	if err := WaitForBackgroundReadyFile(readyPath, cfg.ReadyTimeout); err != nil {
		return 0, err
	}

	return pid, nil
}

func NotifyBackgroundReady(daemonEnvName, readyFileEnvName string) {
	if os.Getenv(daemonEnvName) != "1" {
		return
	}

	if readyPath := strings.TrimSpace(os.Getenv(readyFileEnvName)); readyPath != "" {
		_ = os.WriteFile(readyPath, []byte("OK\n"), 0o600)
	}

	f := os.NewFile(uintptr(3), "lssh_ready")
	if f == nil {
		return
	}
	defer f.Close()

	_, _ = f.Write([]byte("OK\n"))
}

func WaitForBackgroundReadyFile(path string, timeout time.Duration) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			return nil
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("background start failed")
}
