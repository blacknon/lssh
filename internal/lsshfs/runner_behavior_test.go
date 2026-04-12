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
func (f *fakeMountConn) SMBForwardAuth(bindAddr, port, shareName, remote string, creds *SMBCredentials) error {
	f.mu.Lock()
	user := ""
	if creds != nil {
		user = creds.Username
	}
	f.forwardCalls = append(f.forwardCalls, "smbauth:"+port+":"+shareName+":"+remote+":"+user)
	f.mu.Unlock()
	return f.smbErr
}
func (f *fakeMountConn) CheckClientAlive() error { return f.aliveErr }

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

func TestRunnerRunWaitsForSMBMountBeforeReady(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	conn := &fakeMountConn{}
	readyCalled := false
	waitCalled := false
	sigCh := make(chan os.Signal, 1)

	runner := &Runner{
		Host:       "web01",
		RemotePath: "/srv/data",
		MountPoint: "Z:",
		ReadWrite:  true,
		GOOS:       "windows",
		createConnect: func(r *Runner) (mountConn, error) {
			return conn, nil
		},
		waitForMountActive: func(goos, mountpoint string, timeout time.Duration) error {
			waitCalled = true
			if readyCalled {
				t.Fatal("ready notifier called before SMB mount became active")
			}
			return nil
		},
		ReadyNotifier: func() {
			readyCalled = true
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
