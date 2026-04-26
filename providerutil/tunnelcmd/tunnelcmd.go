package tunnelcmd

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type synchronizedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type managedConn struct {
	net.Conn
	cmd    *exec.Cmd
	once   sync.Once
	waitCh <-chan error
}

func (c *managedConn) Close() error {
	var closeErr error
	c.once.Do(func() {
		if c.Conn != nil {
			closeErr = c.Conn.Close()
		}
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		if c.waitCh != nil {
			select {
			case <-c.waitCh:
			case <-time.After(2 * time.Second):
			}
		}
	})
	return closeErr
}

func PickFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("failed to resolve local tcp address")
	}
	return addr.Port, nil
}

func StartAndDial(ctx context.Context, command []string, env map[string]string, localHost string, localPort int, timeout time.Duration) (net.Conn, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("tunnel command is required")
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Env = append([]string{}, os.Environ()...)
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	output := &synchronizedBuffer{}
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
		close(waitCh)
	}()

	target := net.JoinHostPort(localHost, fmt.Sprint(localPort))
	deadline := time.Now().Add(timeout)
	for {
		dialer := &net.Dialer{Timeout: 500 * time.Millisecond}
		conn, err := dialer.DialContext(ctx, "tcp", target)
		if err == nil {
			return &managedConn{Conn: conn, cmd: cmd, waitCh: waitCh}, nil
		}

		select {
		case waitErr := <-waitCh:
			return nil, formatTunnelCommandError("tunnel command exited before ready", waitErr, output.String())
		default:
		}

		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			waitErr := <-waitCh
			return nil, formatTunnelCommandError(fmt.Sprintf("wait tunnel command ready on %s", target), choosePrimaryError(err, waitErr), output.String())
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func choosePrimaryError(primary error, secondary error) error {
	if primary != nil {
		return primary
	}
	return secondary
}

func formatTunnelCommandError(prefix string, err error, output string) error {
	message := prefix
	if err != nil {
		message += ": " + err.Error()
	}
	if snippet := compactCommandOutput(output); snippet != "" {
		message += " (command output: " + snippet + ")"
	}
	return fmt.Errorf("%s", message)
}

func compactCommandOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	output = strings.Join(strings.Fields(output), " ")
	const limit = 280
	if len(output) > limit {
		return output[:limit-3] + "..."
	}
	return output
}
