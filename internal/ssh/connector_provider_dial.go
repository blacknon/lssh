package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"sync"
	"time"

	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/providerapi"
)

type providerRuntimeConn struct {
	reader io.ReadCloser
	writer io.WriteCloser
	cmd    *exec.Cmd

	closeOnce sync.Once
	waitDone  chan error
	stderr    bytes.Buffer
}

func (c *providerRuntimeConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func (c *providerRuntimeConn) Write(p []byte) (int, error) {
	return c.writer.Write(p)
}

func (c *providerRuntimeConn) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		if c.writer != nil {
			_ = c.writer.Close()
		}
		if c.reader != nil {
			_ = c.reader.Close()
		}
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		if err := <-c.waitDone; err != nil && closeErr == nil {
			closeErr = err
		}
	})
	return closeErr
}

func (c *providerRuntimeConn) LocalAddr() net.Addr              { return providerRuntimeAddr("provider-runtime") }
func (c *providerRuntimeConn) RemoteAddr() net.Addr             { return providerRuntimeAddr("provider-runtime") }
func (c *providerRuntimeConn) SetDeadline(time.Time) error      { return nil }
func (c *providerRuntimeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *providerRuntimeConn) SetWriteDeadline(time.Time) error { return nil }

type providerRuntimeAddr string

func (a providerRuntimeAddr) Network() string { return "tcp" }
func (a providerRuntimeAddr) String() string  { return string(a) }

func (r *Run) dialConnectorProviderManagedTarget(_ context.Context, server string, plan providerapi.ConnectorPlan) (net.Conn, error) {
	cmd, _, err := r.Conf.PrepareProviderRuntimeCommand(context.Background(), server, plan, providerapi.MethodConnectorDial, "", "", false)
	if err != nil {
		return nil, err
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	conn := &providerRuntimeConn{
		reader:   stdout,
		writer:   stdin,
		cmd:      cmd,
		waitDone: make(chan error, 1),
	}
	cmd.Stderr = &conn.stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		waitErr := cmd.Wait()
		if waitErr != nil && conn.stderr.Len() > 0 {
			waitErr = fmt.Errorf("%w: %s", waitErr, conn.stderr.String())
		}
		conn.waitDone <- waitErr
	}()
	return conn, nil
}

func (r *Run) createConnectorManagedDialNetConn(ctx context.Context, server string) (net.Conn, error) {
	prepared, err := r.prepareConnectorDialTransport(server, "127.0.0.1", "22")
	if err != nil {
		return nil, err
	}

	switch {
	case prepared.ManagedSSH != nil:
		return r.dialConnectorManagedSSHTarget(ctx, server, "tcp", net.JoinHostPort("127.0.0.1", "22"))
	case prepared.ProviderManagedPlan != nil:
		return r.dialConnectorProviderManagedTarget(ctx, server, *prepared.ProviderManagedPlan)
	default:
		if prepared.PlanKind != "" {
			return nil, fmt.Errorf("server %q connector %q returned unsupported dial plan kind %q", server, prepared.ConnectorName, prepared.PlanKind)
		}
		return nil, fmt.Errorf("server %q connector %q returned unsupported dial runtime", server, prepared.ConnectorName)
	}
}

func managedSSHLike(prepared conf.PreparedConnector) bool {
	return prepared.ManagedSSH != nil || prepared.ProviderManagedPlan != nil
}
