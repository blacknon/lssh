package tunnelcmd

import (
	"net"
	"testing"
	"time"
)

func TestManagedConnCloseDoesNotBlockForeverWhenWaitNeverReturns(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()

	conn := &managedConn{
		Conn:   client,
		waitCh: make(chan error),
	}

	done := make(chan struct{})
	go func() {
		_ = conn.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("managedConn.Close() blocked waiting for tunnel process exit")
	}
}
