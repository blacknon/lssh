package ssh

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/internal/providerapi"
	"github.com/blacknon/lssh/provider/connector/provider-connector-openssh/opensshlib"
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
		prepared, err := r.Conf.PrepareConnector(server, providerapi.ConnectorOperation{Name: "sftp_transport"})
		if err != nil {
			return nil, err
		}
		if !prepared.Supported {
			return nil, fmt.Errorf("connector for %q does not support sftp_transport", server)
		}
		if connectorManagedSSHRuntime(prepared.Plan, r.Conf.ServerConnectorName(server)) {
			return r.createConnectorManagedSFTPHandle(server)
		}
		if prepared.Plan.Kind != "command" {
			return nil, fmt.Errorf("connector plan for %q does not provide command-based sftp transport", server)
		}

		transport, err := opensshlib.StartSFTPTransportPlan(context.Background(), prepared.Plan)
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
