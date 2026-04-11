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
	defaultMountRetryCount    = 20
	defaultMountRetryDelay    = 500 * time.Millisecond
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
	if r.GOOS == "" {
		r.GOOS = currentGOOS
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

	if backend != BackendSMB {
		if err := os.MkdirAll(r.MountPoint, 0o755); err != nil {
			return err
		}
	}

	run := &lsshssh.Run{
		ServerList:            []string{r.Host},
		Conf:                  r.Config,
		ControlMasterOverride: r.ControlMasterOverride,
	}
	run.CreateAuthMethodMap()

	connect, err := run.CreateSshConnectDirect(r.Host)
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
		spec, err := mountCommand(r.GOOS, r.MountPoint, port, "")
		if err != nil {
			return err
		}
		if err := r.mountWithRetry(spec, serveErrCh); err != nil {
			return err
		}
	case BackendSMB:
		go func() {
			serveErrCh <- connect.SMBForward("127.0.0.1", "445", defaultSMBShareName, r.RemotePath)
		}()
		spec, err := mountCommand(r.GOOS, r.MountPoint, 445, defaultSMBShareName)
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

	if r.ReadyNotifier != nil {
		r.ReadyNotifier()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	ticker := time.NewTicker(defaultAliveCheckInterval)
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
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	return cmd.Run()
}
