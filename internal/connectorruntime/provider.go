package connectorruntime

import (
	"context"

	"github.com/blacknon/lssh/providerapi"
)

type ShellRequest struct {
	Server         string
	Plan           providerapi.ConnectorPlan
	LocalRCCommand string
	StartupMarker  string
	Stream         StreamConfig
}

type ExecRequest struct {
	Server string
	Plan   providerapi.ConnectorPlan
	Stream StreamConfig
}

type Executor interface {
	RunShell(ctx context.Context, request ShellRequest) error
	RunExec(ctx context.Context, request ExecRequest) (int, error)
}
