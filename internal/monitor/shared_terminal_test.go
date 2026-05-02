package monitor

import (
	"fmt"
	"io"
	"sync/atomic"
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/mux"
	sshrun "github.com/blacknon/lssh/internal/ssh"
)

type testCloser struct {
	count *int32
	err   error
}

func (c testCloser) Close() error {
	if c.count != nil {
		atomic.AddInt32(c.count, 1)
	}
	return c.err
}

func TestCreateTopTerminalSessionUsesSharedFactoryWhenEnabled(t *testing.T) {
	t.Parallel()

	sharedSession := &mux.RemoteSession{Server: "shared"}
	ownedSession := &mux.RemoteSession{Server: "owned"}
	sharedCalled := false

	m := &Monitor{
		shareConnect: true,
		termFactory: func(string, int, int) (*mux.RemoteSession, error) {
			return ownedSession, nil
		},
		sharedTermFactory: func(string, int, int) (*mux.RemoteSession, error) {
			sharedCalled = true
			return sharedSession, nil
		},
	}

	session, err := m.createTopTerminalSession("host1", 80, 24)
	if err != nil {
		t.Fatalf("createTopTerminalSession returned error: %v", err)
	}
	if !sharedCalled {
		t.Fatal("shared factory was not called")
	}
	if session != sharedSession {
		t.Fatalf("session = %v, want shared session", session)
	}
}

func TestCreateTopTerminalSessionUsesOwnedFactoryByDefault(t *testing.T) {
	t.Parallel()

	sharedSession := &mux.RemoteSession{Server: "shared"}
	ownedSession := &mux.RemoteSession{Server: "owned"}
	sharedCalled := false

	m := &Monitor{
		shareConnect: false,
		termFactory: func(string, int, int) (*mux.RemoteSession, error) {
			return ownedSession, nil
		},
		sharedTermFactory: func(string, int, int) (*mux.RemoteSession, error) {
			sharedCalled = true
			return sharedSession, nil
		},
	}

	session, err := m.createTopTerminalSession("host1", 80, 24)
	if err != nil {
		t.Fatalf("createTopTerminalSession returned error: %v", err)
	}
	if sharedCalled {
		t.Fatal("shared factory should not be called")
	}
	if session != ownedSession {
		t.Fatalf("session = %v, want owned session", session)
	}
}

func TestNewSharedTerminalCloseFuncDoesNotCloseClientWhenShared(t *testing.T) {
	t.Parallel()

	_, writer := io.Pipe()
	var terminalClosed int32
	var clientClosed int32

	closeFn := newSharedTerminalCloseFunc(
		writer,
		nil,
		testCloser{count: &terminalClosed},
		testCloser{count: &clientClosed},
		false,
	)

	if err := closeFn(); err != nil {
		t.Fatalf("closeFn() returned error: %v", err)
	}
	if got := atomic.LoadInt32(&terminalClosed); got != 1 {
		t.Fatalf("terminal close count = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&clientClosed); got != 0 {
		t.Fatalf("client close count = %d, want 0", got)
	}
}

func TestNewSharedTerminalCloseFuncClosesClientWhenOwned(t *testing.T) {
	t.Parallel()

	_, writer := io.Pipe()
	var terminalClosed int32
	var clientClosed int32

	closeFn := newSharedTerminalCloseFunc(
		writer,
		nil,
		testCloser{count: &terminalClosed},
		testCloser{count: &clientClosed},
		true,
	)

	if err := closeFn(); err != nil {
		t.Fatalf("closeFn() returned error: %v", err)
	}
	if got := atomic.LoadInt32(&terminalClosed); got != 1 {
		t.Fatalf("terminal close count = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&clientClosed); got != 1 {
		t.Fatalf("client close count = %d, want 1", got)
	}
}

func TestNewSharedTerminalCloseFuncReturnsFirstError(t *testing.T) {
	t.Parallel()

	_, writer := io.Pipe()
	wantErr := fmt.Errorf("boom")

	closeFn := newSharedTerminalCloseFunc(
		writer,
		testCloser{err: wantErr},
		testCloser{},
		testCloser{},
		true,
	)

	if err := closeFn(); err == nil || err.Error() != wantErr.Error() {
		t.Fatalf("closeFn() error = %v, want %v", err, wantErr)
	}
}

func TestSharedTerminalLocalRCEnabled(t *testing.T) {
	t.Parallel()

	serverConf := conf.ServerConfig{LocalRcUse: "yes"}
	if !sharedTerminalLocalRCEnabled(serverConf, nil) {
		t.Fatal("sharedTerminalLocalRCEnabled() = false, want true from config")
	}

	if sharedTerminalLocalRCEnabled(serverConf, &sshrun.Run{IsNotBashrc: true}) {
		t.Fatal("sharedTerminalLocalRCEnabled() = true, want false with --not-localrc")
	}

	serverConf.LocalRcUse = "no"
	if !sharedTerminalLocalRCEnabled(serverConf, &sshrun.Run{IsBashrc: true}) {
		t.Fatal("sharedTerminalLocalRCEnabled() = false, want true with --localrc")
	}
}
