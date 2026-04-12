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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	conf "github.com/blacknon/lssh/internal/config"
	lsshssh "github.com/blacknon/lssh/internal/ssh"
)

const (
	defaultAliveCheckInterval = 5 * time.Second
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
	GOOS                  string
	ControlMasterOverride *bool
	ReadyNotifier         func()
	Stdout                io.Writer
	Stderr                io.Writer
	execCommand           func(name string, args ...string) *exec.Cmd
	createConnect         func(*Runner) (mountConn, error)
	waitForMountActive    func(goos, mountpoint string, timeout time.Duration) error
	aliveCheckInterval    time.Duration
	signalCh              <-chan os.Signal
}

type mountConn interface {
	Close() error
	FUSEForward(local, remote string) error
	NFSForward(bindAddr, port, remote string) error
	SMBForward(bindAddr, port, shareName, remote string) error
	CheckClientAlive() error
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

	backend, err := backendForGOOS(r.GOOS)
	if err != nil {
		return err
	}

	mountpoint, err := normalizeMountPoint(r.GOOS, r.MountPoint)
	if err != nil {
		return err
	}
	r.MountPoint = mountpoint

	if backend != BackendWinFsp {
		if err := os.MkdirAll(r.MountPoint, 0o755); err != nil {
			return err
		}
	}

	connect, err := r.createConnect(r)
	if err != nil {
		return err
	}
	defer connect.Close()

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

	serveErrCh := make(chan error, 1)
	switch backend {
	case BackendFUSE:
		go func() {
			serveErrCh <- connect.FUSEForward(r.MountPoint, r.RemotePath)
		}()
	case BackendNFS:
		port, err := pickFreePort()
		if err != nil {
			return err
		}
		go func() {
			serveErrCh <- connect.NFSForward("127.0.0.1", strconv.Itoa(port), r.RemotePath)
		}()
		spec, err := mountCommand(r.GOOS, r.MountPoint, port, "", nil)
		if err != nil {
			return err
		}
		if err := r.mountWithRetry(spec, serveErrCh); err != nil {
			return err
		}
	case BackendWinFsp:
		spec, err := r.winFspMountCommand()
		if err != nil {
			return err
		}
		if err := r.mountWithRetry(spec, serveErrCh); err != nil {
			return err
		}
	}

	if backend == BackendFUSE {
		select {
		case err := <-serveErrCh:
			if err != nil {
				return err
			}
			return nil
		case <-time.After(defaultMountRetryDelay):
		}
	}

	if err := r.waitForMountActive(r.GOOS, r.MountPoint, defaultMountActiveTimeout); err != nil {
		return err
	}

	if r.ReadyNotifier != nil {
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

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			if commands, err := unmountCommands(r.GOOS, r.MountPoint); err == nil {
				for _, command := range commands {
					if err := r.runCommand(command); err == nil {
						break
					}
				}
			}
			_ = connect.Close()
		})
	}

	for {
		select {
		case err := <-serveErrCh:
			cleanup()
			return err
		case <-ticker.C:
			if err := connect.CheckClientAlive(); err != nil {
				fmt.Fprintf(r.Stderr, "Information   : ssh connection lost, unmounting %s\n", r.MountPoint)
				cleanup()
				return nil
			}
		case <-sigCh:
			cleanup()
			return nil
		}
	}
}

func createMountConn(r *Runner) (mountConn, error) {
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
		if err := r.runCommand(spec); err == nil {
			return nil
		} else {
			lastErr = err
		}

		select {
		case err := <-serveErrCh:
			if err != nil {
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
	return cmd.Run()
}

func (r *Runner) winFspMountCommand() (CommandSpec, error) {
	server, ok := r.Config.Server[r.Host]
	if !ok {
		return CommandSpec{}, fmt.Errorf("host not found in config: %s", r.Host)
	}
	if strings.TrimSpace(server.Addr) == "" || strings.TrimSpace(server.User) == "" {
		return CommandSpec{}, fmt.Errorf("windows winfsp backend requires host addr and user")
	}
	if server.Pass != "" || len(server.Passes) > 0 {
		return CommandSpec{}, fmt.Errorf("windows winfsp backend currently supports key/agent auth only")
	}

	remoteSpec := fmt.Sprintf("%s@%s:%s", server.User, server.Addr, r.RemotePath)
	args := []string{remoteSpec, r.MountPoint}

	if strings.TrimSpace(server.Port) != "" && server.Port != "22" {
		args = append(args, "-p", server.Port)
	}
	if key := firstKeyPath(server); key != "" {
		args = append(args, "-o", "IdentityFile="+filepath.Clean(key))
	}
	if len(server.KnownHostsFiles) > 0 && strings.TrimSpace(server.KnownHostsFiles[0]) != "" {
		args = append(args, "-o", "UserKnownHostsFile="+filepath.Clean(server.KnownHostsFiles[0]))
	}
	if !server.CheckKnownHosts {
		args = append(args, "-o", "StrictHostKeyChecking=no")
	}

	return CommandSpec{Name: "sshfs.exe", Args: args}, nil
}

func firstKeyPath(server conf.ServerConfig) string {
	if strings.TrimSpace(server.Key) != "" {
		return server.Key
	}
	for _, key := range server.Keys {
		pair := strings.SplitN(key, "::", 2)
		if strings.TrimSpace(pair[0]) != "" {
			return pair[0]
		}
	}
	return ""
}
