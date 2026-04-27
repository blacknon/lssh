package connectorruntime

import (
	"context"
	"net"

	"github.com/blacknon/lssh/providerapi"
)

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type DialerFunc func(ctx context.Context, network, address string) (net.Conn, error)

func (f DialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

type ShellRequest struct {
	Server         string
	Plan           providerapi.ConnectorPlan
	LocalRCCommand string
	StartupMarker  string
	Dialer         Dialer
	Stream         StreamConfig
}

type ExecRequest struct {
	Server string
	Plan   providerapi.ConnectorPlan
	Dialer Dialer
	Stream StreamConfig
}

type Executor interface {
	RunShell(ctx context.Context, request ShellRequest) error
	RunExec(ctx context.Context, request ExecRequest) (int, error)
}
