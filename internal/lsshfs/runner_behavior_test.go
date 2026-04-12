package lsshfs

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

type fakeMountConn struct {
	fuseErr      error
	nfsErr       error
	smbErr       error
	aliveErr     error
	aliveErrs    []error
	aliveChecks  int
	closeCount   int
	forwardCalls []string
	fuseBlock    <-chan struct{}
	mu           sync.Mutex
}

func (f *fakeMountConn) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeCount++
	return nil
}
func (f *fakeMountConn) FUSEForward(local, remote string) error {
	f.mu.Lock()
	f.forwardCalls = append(f.forwardCalls, "fuse:"+local+":"+remote)
	f.mu.Unlock()
	if f.fuseBlock != nil {
		<-f.fuseBlock
	}
	return f.fuseErr
}
func (f *fakeMountConn) NFSForward(bindAddr, port, remote string) error {
	f.mu.Lock()
	f.forwardCalls = append(f.forwardCalls, "nfs:"+port+":"+remote)
	f.mu.Unlock()
	return f.nfsErr
}
func (f *fakeMountConn) SMBForward(bindAddr, port, shareName, remote string) error {
	f.mu.Lock()
	f.forwardCalls = append(f.forwardCalls, "smb:"+port+":"+shareName+":"+remote)
	f.mu.Unlock()
	return f.smbErr
}
func (f *fakeMountConn) CheckClientAlive() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.aliveChecks++
	if len(f.aliveErrs) > 0 {
		err := f.aliveErrs[0]
		f.aliveErrs = f.aliveErrs[1:]
		return err
	}
	return f.aliveErr
}

func TestRunnerRunWaitsForMountBeforeReady(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	done := make(chan struct{})
	conn := &fakeMountConn{fuseBlock: done}
	readyCalled := false
	waitCalled := false
	sigCh := make(chan os.Signal, 1)

	runner := &Runner{
		Host:       "web01",
		RemotePath: "/srv/data",
		MountPoint: t.TempDir(),
		ReadWrite:  true,
		GOOS:       "linux",
		createConnect: func(r *Runner) (mountConn, error) {
			return conn, nil
		},
		waitForMountActive: func(goos, mountpoint string, timeout time.Duration) error {
			waitCalled = true
			if readyCalled {
				t.Fatal("ready notifier called before mount became active")
			}
			return nil
		},
		ReadyNotifier: func() {
			readyCalled = true
			close(done)
			sigCh <- syscall.SIGTERM
		},
		signalCh: sigCh,
		execCommand: func(name string, args ...string) *exec.Cmd {
			return exec.Command("sh", "-c", "true")
		},
	}

	if err := runner.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !waitCalled || !readyCalled {
		t.Fatalf("waitCalled=%v readyCalled=%v", waitCalled, readyCalled)
	}
}

func TestRunnerRunReturnsErrorWhenMountNeverBecomesActive(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	done := make(chan struct{})
	runner := &Runner{
		Host:       "web01",
		RemotePath: "/srv/data",
		MountPoint: t.TempDir(),
		ReadWrite:  true,
		GOOS:       "linux",
		createConnect: func(r *Runner) (mountConn, error) {
			return &fakeMountConn{fuseBlock: done}, nil
		},
		waitForMountActive: func(goos, mountpoint string, timeout time.Duration) error {
			return errors.New("not active")
		},
	}

	err := runner.Run()
	if err == nil || !strings.Contains(err.Error(), "not active") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunnerRunUnmountsOnDisconnect(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	conn := &fakeMountConn{aliveErr: errors.New("lost"), fuseBlock: make(chan struct{})}
	var commands []string
	var stderr bytes.Buffer
	sigCh := make(chan os.Signal, 1)

	runner := &Runner{
		Host:                "web01",
		RemotePath:          "/srv/data",
		MountPoint:          t.TempDir(),
		ReadWrite:           true,
		GOOS:                "linux",
		aliveCheckInterval:  time.Millisecond,
		aliveFailureLimit:   1,
		signalCh:            sigCh,
		Stderr:              &stderr,
		createConnect: func(r *Runner) (mountConn, error) {
			return conn, nil
		},
		waitForMountActive: func(goos, mountpoint string, timeout time.Duration) error {
			return nil
		},
		execCommand: func(name string, args ...string) *exec.Cmd {
			commands = append(commands, name+" "+strings.Join(args, " "))
			return exec.Command("sh", "-c", "true")
		},
	}

	if err := runner.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(commands) == 0 {
		t.Fatal("no unmount command executed")
	}
	if !strings.Contains(stderr.String(), "ssh connection lost") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunnerRunReturnsUnsupportedOnWindows(t *testing.T) {
	runner := &Runner{
		Host:       "web01",
		RemotePath: "/srv/data",
		MountPoint: "Z:",
		ReadWrite:  true,
		GOOS:       "windows",
	}

	err := runner.Run()
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "does not support windows") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunnerRunDoesNotUnmountOnSingleAliveFailure(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	done := make(chan struct{})
	conn := &fakeMountConn{
		aliveErrs: []error{errors.New("temporary"), nil, nil},
		fuseBlock: done,
	}
	sigCh := make(chan os.Signal, 1)
	var stderr bytes.Buffer

	runner := &Runner{
		Host:               "web01",
		RemotePath:         "/srv/data",
		MountPoint:         t.TempDir(),
		ReadWrite:          true,
		GOOS:               "linux",
		aliveCheckInterval: time.Millisecond,
		aliveFailureLimit:  2,
		signalCh:           sigCh,
		Stderr:             &stderr,
		createConnect: func(r *Runner) (mountConn, error) {
			return conn, nil
		},
		waitForMountActive: func(goos, mountpoint string, timeout time.Duration) error {
			return nil
		},
		execCommand: func(name string, args ...string) *exec.Cmd {
			return exec.Command("sh", "-c", "true")
		},
	}

	go func() {
		deadline := time.Now().Add(1200 * time.Millisecond)
		for time.Now().Before(deadline) {
			conn.mu.Lock()
			checks := conn.aliveChecks
			conn.mu.Unlock()
			if checks >= 2 {
				break
			}
			time.Sleep(time.Millisecond)
		}
		close(done)
		sigCh <- syscall.SIGTERM
	}()

	if err := runner.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if conn.closeCount == 0 {
		t.Fatalf("expected connection to close during cleanup")
	}
	if conn.aliveChecks == 0 {
		t.Fatalf("expected alive check to run")
	}
	if strings.Contains(stderr.String(), "ssh connection lost") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
