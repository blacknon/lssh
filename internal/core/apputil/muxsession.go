package apputil

import (
	"os"
	"os/exec"
	"time"

	lsmuxsession "github.com/blacknon/lssh/internal/core/lsmuxsession"
)

type PersistentSessionArgsConfig struct {
	AllArgs     []string
	BareFlags   map[string]bool
	ValueFlags  map[string]bool
	DaemonFlag  string
	SessionFlag string
	SocketFlag  string
	Name        string
	SocketPath  string
}

func BuildPersistentSessionArgs(cfg PersistentSessionArgsConfig) []string {
	args := FilterCLIArgs(cfg.AllArgs, cfg.BareFlags, cfg.ValueFlags)
	args = append(args, cfg.DaemonFlag, cfg.SessionFlag, cfg.Name)
	if cfg.SocketFlag != "" && cfg.SocketPath != "" {
		args = append(args, cfg.SocketFlag, cfg.SocketPath)
	}

	return args
}

type MuxSessionDaemonConfig struct {
	Name       string
	ConfigPath string
	SocketPath string
	Exe        string
	Args       []string
	Env        []string
	Ready      func()
}

func RunMuxSessionDaemon(cfg MuxSessionDaemonConfig) error {
	daemon := &lsmuxsession.Daemon{
		Name:       cfg.Name,
		ConfigPath: cfg.ConfigPath,
		SocketPath: cfg.SocketPath,
		Exe:        cfg.Exe,
		Args:       cfg.Args,
		Env:        cfg.Env,
	}

	return daemon.Run(cfg.Ready)
}

type MuxSessionSpawnConfig struct {
	GOOS          string
	Name          string
	DaemonEnvName string
	Args          []string
	Stdout        *os.File
	Stderr        *os.File
	Prepare       func(*exec.Cmd)
	Resolve       func(string) (lsmuxsession.Session, error)
}

func SpawnMuxSession(cfg MuxSessionSpawnConfig) (lsmuxsession.Session, error) {
	_, err := StartBackgroundProcess(BackgroundLaunchConfig{
		GOOS:          cfg.GOOS,
		Args:          cfg.Args,
		DaemonEnvName: cfg.DaemonEnvName,
		Stdout:        cfg.Stdout,
		Stderr:        cfg.Stderr,
		Prepare:       cfg.Prepare,
	})
	if err != nil {
		return lsmuxsession.Session{}, err
	}

	time.Sleep(200 * time.Millisecond)
	return cfg.Resolve(cfg.Name)
}
