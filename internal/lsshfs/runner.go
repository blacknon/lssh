// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsshfs

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	conf "github.com/blacknon/lssh/internal/config"
	lsshssh "github.com/blacknon/lssh/internal/ssh"
)

const (
	defaultAliveCheckInterval = 5 * time.Second
	defaultAliveFailureLimit  = 3
	defaultMountRetryCount    = 20
	defaultMountRetryDelay    = 500 * time.Millisecond
	defaultMountActiveTimeout = 10 * time.Second
)

type Runner struct {
	Config                conf.Config
	Host                  string
	RemotePath            string
	MountPoint            string
	ReadWrite             bool
	MountOptions          []string
	GOOS                  string
	ControlMasterOverride *bool
	ReadyNotifier         func()
	Stdout                io.Writer
	Stderr                io.Writer
	execCommand           func(name string, args ...string) *exec.Cmd
	createConnect         func(*Runner) (mountConn, error)
	waitForMountActive    func(goos, mountpoint string, timeout time.Duration) error
	aliveCheckInterval    time.Duration
	aliveFailureLimit     int
	signalCh              <-chan os.Signal
}

type mountConn interface {
	Close() error
	FUSEForward(local, remote string) error
	NFSForward(bindAddr, port, remote string) error
	SMBForward(bindAddr, port, shareName, remote string) error
	CheckClientAlive() error
}

func (r *Runner) debugEnabled() bool {
	switch os.Getenv("LSSHFS_DEBUG") {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	default:
		return false
	}
}

func (r *Runner) debugf(format string, args ...interface{}) {
	if !r.debugEnabled() {
		return
	}
	if path := os.Getenv("LSSHFS_DEBUG_LOG"); path != "" {
		if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			_, _ = fmt.Fprintf(f, "DEBUG lsshfs: "+format+"\n", args...)
			_ = f.Close()
			return
		}
	}
	_, _ = fmt.Fprintf(r.Stderr, "DEBUG lsshfs: "+format+"\n", args...)
}

func (r *Runner) Run() error {
	if r.Stdout == nil {
		r.Stdout = os.Stdout
	}
	if r.Stderr == nil {
		r.Stderr = os.Stderr
	}
	if r.execCommand == nil {
		r.execCommand = exec.Command
	}
	if r.createConnect == nil {
		r.createConnect = createMountConn
	}
	if r.waitForMountActive == nil {
		r.waitForMountActive = waitForMountActive
	}
	if r.GOOS == "" {
		r.GOOS = currentGOOS
	}
	if r.aliveCheckInterval == 0 {
		r.aliveCheckInterval = defaultAliveCheckInterval
	}
	if r.aliveFailureLimit == 0 {
		r.aliveFailureLimit = defaultAliveFailureLimit
	}

	backend, err := backendForGOOS(r.GOOS)
	if err != nil {
		return err
	}
	if r.Config.ServerUsesConnector(r.Host) && (r.GOOS == "linux" || r.GOOS == "darwin") {
		// Connector-backed mounts use the Go-side SFTP/FUSE backend on Unix-like systems.
		backend = BackendFUSE
	}
	r.debugf("start host=%s remote_path=%s mountpoint=%s goos=%s backend=%s", r.Host, r.RemotePath, r.MountPoint, r.GOOS, backend)

	mountpoint, err := normalizeMountPoint(r.GOOS, r.MountPoint)
	if err != nil {
		return err
	}
	r.MountPoint = mountpoint

	if backend != BackendNFS || r.GOOS != "windows" {
		if err := os.MkdirAll(r.MountPoint, 0o755); err != nil {
			return err
		}
	}

	connect, err := r.createConnect(r)
	if err != nil {
		return err
	}
	defer connect.Close()
	r.debugf("ssh connection established host=%s", r.Host)

	record := MountRecord{
		Host:       r.Host,
		RemotePath: r.RemotePath,
		MountPoint: r.MountPoint,
		Backend:    string(backend),
		PID:        os.Getpid(),
		ReadWrite:  r.ReadWrite,
	}
	if err := writeMountRecord(record); err != nil {
		return err
	}
	defer removeMountRecord(r.MountPoint)
	r.debugf("mount record written mountpoint=%s", r.MountPoint)

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			r.debugf("cleanup start mountpoint=%s", r.MountPoint)
			if commands, err := unmountCommands(r.GOOS, r.MountPoint); err == nil {
				for _, command := range commands {
					r.debugf("cleanup command name=%s args=%v", command.Name, command.Args)
					if err := r.runCommand(command); err == nil {
						r.debugf("cleanup command succeeded name=%s", command.Name)
						break
					}
				}
			}
			_ = connect.Close()
			r.debugf("cleanup complete mountpoint=%s", r.MountPoint)
		})
	}

	serveErrCh := make(chan error, 1)
	switch backend {
	case BackendFUSE:
		r.debugf("starting FUSE forward mountpoint=%s remote_path=%s", r.MountPoint, r.RemotePath)
		go func() {
			serveErrCh <- connect.FUSEForward(r.MountPoint, r.RemotePath)
		}()
	case BackendNFS:
		port := 0
		if r.GOOS == "windows" {
			if _, err := exec.LookPath("mount"); err != nil {
				return fmt.Errorf("windows nfs client is not installed or mount command is unavailable")
			}
			port = 2049
		} else {
			var err error
			port, err = pickFreePort()
			if err != nil {
				return err
			}
		}
		go func() {
			serveErrCh <- connect.NFSForward("127.0.0.1", strconv.Itoa(port), r.RemotePath)
		}()
		spec, err := mountCommand(r.GOOS, r.MountPoint, port, "", nil, r.MountOptions)
		if err != nil {
			cleanup()
			return err
		}
		if err := r.mountWithRetry(spec, serveErrCh); err != nil {
			cleanup()
			return err
		}
	}

	if backend == BackendFUSE {
		select {
		case err := <-serveErrCh:
			if err != nil {
				r.debugf("serve loop exited before mount became active err=%v", err)
				cleanup()
				return err
			}
			return nil
		case <-time.After(defaultMountRetryDelay):
		}
	}

	if err := r.waitForMountActive(r.GOOS, r.MountPoint, defaultMountActiveTimeout); err != nil {
		r.debugf("mount did not become active mountpoint=%s err=%v", r.MountPoint, err)
		cleanup()
		return err
	}
	r.debugf("mount active mountpoint=%s", r.MountPoint)

	if r.ReadyNotifier != nil {
		r.debugf("notify ready mountpoint=%s", r.MountPoint)
		r.ReadyNotifier()
	}

	sigCh := r.signalCh
	if sigCh == nil {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(ch)
		sigCh = ch
	}

	ticker := time.NewTicker(r.aliveCheckInterval)
	defer ticker.Stop()

	aliveFailures := 0

	for {
		select {
		case err := <-serveErrCh:
			r.debugf("serve loop exited mountpoint=%s err=%v", r.MountPoint, err)
			cleanup()
			return err
		case <-ticker.C:
			if err := connect.CheckClientAlive(); err != nil {
				aliveFailures++
				r.debugf("alive check failed count=%d limit=%d err=%v", aliveFailures, r.aliveFailureLimit, err)
				if aliveFailures >= r.aliveFailureLimit {
					fmt.Fprintf(r.Stderr, "Information   : ssh connection lost, unmounting %s\n", r.MountPoint)
					cleanup()
					return nil
				}
				continue
			}
			aliveFailures = 0
			r.debugf("alive check ok mountpoint=%s", r.MountPoint)
		case <-sigCh:
			r.debugf("signal received mountpoint=%s", r.MountPoint)
			cleanup()
			return nil
		}
	}
}

func createMountConn(r *Runner) (mountConn, error) {
	goos := r.GOOS
	if goos == "" {
		goos = currentGOOS
	}

	if r.Config.ServerUsesConnector(r.Host) {
		if goos != "linux" && goos != "darwin" {
			return nil, fmt.Errorf("connector-backed lsshfs currently supports linux and macos only")
		}

		run := &lsshssh.Run{
			ServerList:            []string{r.Host},
			Conf:                  r.Config,
			ControlMasterOverride: r.ControlMasterOverride,
		}
		run.CreateAuthMethodMap()

		handle, err := run.CreateSFTPClientHandle(r.Host)
		if err != nil {
			return nil, err
		}
		if handle == nil || handle.Client == nil {
			if handle != nil && handle.Closer != nil {
				_ = handle.Closer.Close()
			}
			return nil, fmt.Errorf("connector-backed sftp client is not available")
		}

		return newSFTPMountConn(handle, r.ReadWrite)
	}

	run := &lsshssh.Run{
		ServerList:            []string{r.Host},
		Conf:                  r.Config,
		ControlMasterOverride: r.ControlMasterOverride,
	}
	run.CreateAuthMethodMap()

	connect, err := run.CreateSshConnectDirect(r.Host)
	if err != nil {
		return nil, err
	}
	return connect, nil
}

func (r *Runner) mountWithRetry(spec CommandSpec, serveErrCh <-chan error) error {
	var lastErr error

	for i := 0; i < defaultMountRetryCount; i++ {
		r.debugf("mount attempt=%d name=%s args=%v", i+1, spec.Name, spec.Args)
		if err := r.runCommand(spec); err == nil {
			r.debugf("mount attempt=%d succeeded", i+1)
			return nil
		} else {
			lastErr = err
			r.debugf("mount attempt=%d failed err=%v", i+1, err)
		}

		select {
		case err := <-serveErrCh:
			if err != nil {
				r.debugf("serve loop exited during mount retry err=%v", err)
				return err
			}
			return nil
		default:
		}

		time.Sleep(defaultMountRetryDelay)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("mount failed")
	}

	return lastErr
}

func (r *Runner) runCommand(spec CommandSpec) error {
	cmd := r.execCommand(spec.Name, spec.Args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	r.debugf("run command name=%s args=%v", spec.Name, spec.Args)
	return cmd.Run()
}
