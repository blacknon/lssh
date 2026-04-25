package opensshlib

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"

	"github.com/blacknon/lssh/providerapi"
	"github.com/pkg/sftp"
)

type SFTPTransport struct {
	Client *sftp.Client

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr bytes.Buffer
}

func StartSFTPTransport(ctx context.Context, cfg Config) (*SFTPTransport, error) {
	plan := BuildSFTPTransportPlan(cfg)
	return StartSFTPTransportPlan(ctx, plan)
}

func StartSFTPTransportPlan(ctx context.Context, plan providerapi.ConnectorPlan) (*SFTPTransport, error) {
	if plan.Kind != "command" {
		return nil, fmt.Errorf("openssh sftp transport requires command plan, got %q", plan.Kind)
	}
	if strings.TrimSpace(plan.Program) == "" {
		return nil, fmt.Errorf("openssh sftp transport requires command program")
	}

	return startSFTPTransportCommand(ctx, plan.Program, plan.Args, envMapToSlice(plan.Env))
}

func startSFTPTransportCommand(ctx context.Context, program string, args []string, env []string) (*SFTPTransport, error) {
	cmd := exec.CommandContext(ctx, program, args...)
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
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

	transport := &SFTPTransport{
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
			return nil, fmt.Errorf("failed to start openssh sftp transport: %w: %s", err, stderr)
		}
		return nil, fmt.Errorf("failed to start openssh sftp transport: %w", err)
	}
	transport.Client = client

	return transport, nil
}

func (t *SFTPTransport) Close() error {
	var errs []string

	if t.Client != nil {
		if err := t.Client.Close(); err != nil {
			errs = append(errs, err.Error())
		}
		t.Client = nil
	}
	if t.stdin != nil {
		if err := t.stdin.Close(); err != nil && !isIgnorablePipeClose(err) {
			errs = append(errs, err.Error())
		}
		t.stdin = nil
	}
	if t.stdout != nil {
		if err := t.stdout.Close(); err != nil && !isIgnorablePipeClose(err) {
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

func isIgnorablePipeClose(err error) bool {
	return errors.Is(err, io.ErrClosedPipe) || strings.Contains(strings.ToLower(err.Error()), "file already closed")
}

func envMapToSlice(env map[string]string) []string {
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
