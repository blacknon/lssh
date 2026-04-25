package ssh

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"

	"github.com/blacknon/go-sshlib"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/pkg/sftp"
)

type SFTPClientHandle struct {
	Client     *sftp.Client
	Closer     io.Closer
	SSHConnect *sshlib.Connect
}

// CreateSFTPClient creates an SFTP client for the target server.
// Connector-backed servers may provide an alternative transport such as
// `ssh -s sftp`; traditional servers continue to use go-sshlib directly.
func (r *Run) CreateSFTPClient(server string) (*sftp.Client, io.Closer, error) {
	handle, err := r.CreateSFTPClientHandle(server)
	if err != nil {
		return nil, nil, err
	}
	return handle.Client, handle.Closer, nil
}

func (r *Run) CreateSFTPClientHandle(server string) (*SFTPClientHandle, error) {
	if r.Conf.ServerUsesConnector(server) {
		prepared, err := r.prepareConnectorOperation(server, conf.ConnectorOperation{Name: "sftp_transport"})
		if err != nil {
			return nil, err
		}
		if !prepared.Supported {
			return nil, fmt.Errorf("connector for %q does not support sftp_transport", server)
		}
		if prepared.ManagedSSH != nil {
			return r.createConnectorManagedSFTPHandle(server)
		}
		if prepared.Command == nil {
			return nil, fmt.Errorf("connector plan for %q does not provide command-based sftp transport", server)
		}

		transport, err := startSFTPTransportPlan(context.Background(), *prepared.Command)
		if err != nil {
			return nil, err
		}
		return &SFTPClientHandle{
			Client: transport.Client,
			Closer: transport,
		}, nil
	}

	conn, err := r.CreateSshConnectDirect(server)
	if err != nil {
		return nil, err
	}
	if conn == nil || conn.Client == nil {
		return nil, fmt.Errorf("ssh client is not available for sftp")
	}

	client, err := sftp.NewClient(conn.Client)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	return &SFTPClientHandle{
		Client:     client,
		Closer:     multiCloser{closers: []io.Closer{client, conn}},
		SSHConnect: conn,
	}, nil
}

type multiCloser struct {
	closers []io.Closer
}

func (m multiCloser) Close() error {
	var errs []string

	for i := len(m.closers) - 1; i >= 0; i-- {
		if m.closers[i] == nil {
			continue
		}
		if err := m.closers[i].Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

var _ io.Closer = (*sshlib.Connect)(nil)

type commandSFTPTransport struct {
	Client *sftp.Client

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr strings.Builder
}

func startSFTPTransportPlan(ctx context.Context, plan conf.ConnectorCommandPlan) (*commandSFTPTransport, error) {
	if strings.TrimSpace(plan.Program) == "" {
		return nil, fmt.Errorf("sftp transport requires command program")
	}

	cmd := exec.CommandContext(ctx, plan.Program, plan.Args...)
	if len(plan.Env) > 0 {
		cmd.Env = append(cmd.Environ(), envMapToSortedSlice(plan.Env)...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}

	transport := &commandSFTPTransport{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
	}
	cmd.Stderr = &transport.stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}

	client, err := sftp.NewClientPipe(stdout, stdin)
	if err != nil {
		_ = transport.Close()
		if stderr := strings.TrimSpace(transport.stderr.String()); stderr != "" {
			return nil, fmt.Errorf("failed to start sftp transport: %w: %s", err, stderr)
		}
		return nil, fmt.Errorf("failed to start sftp transport: %w", err)
	}
	transport.Client = client

	return transport, nil
}

func (t *commandSFTPTransport) Close() error {
	var errs []string

	if t.Client != nil {
		if err := t.Client.Close(); err != nil {
			errs = append(errs, err.Error())
		}
		t.Client = nil
	}
	if t.stdin != nil {
		if err := t.stdin.Close(); err != nil && !isIgnorableSFTPPipeClose(err) {
			errs = append(errs, err.Error())
		}
		t.stdin = nil
	}
	if t.stdout != nil {
		if err := t.stdout.Close(); err != nil && !isIgnorableSFTPPipeClose(err) {
			errs = append(errs, err.Error())
		}
		t.stdout = nil
	}
	if t.cmd != nil {
		if err := t.cmd.Wait(); err != nil {
			if stderr := strings.TrimSpace(t.stderr.String()); stderr != "" {
				errs = append(errs, fmt.Sprintf("%v: %s", err, stderr))
			} else {
				errs = append(errs, err.Error())
			}
		}
		t.cmd = nil
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

func isIgnorableSFTPPipeClose(err error) bool {
	return err == io.ErrClosedPipe || strings.Contains(strings.ToLower(err.Error()), "file already closed")
}

func envMapToSortedSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+env[key])
	}

	return result
}
