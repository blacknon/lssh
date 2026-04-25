package psrplib

import (
	"context"
	"fmt"

	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/psrp"
	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/wsman"
	"github.com/google/uuid"
)

type RunspacePool struct {
	ID    uuid.UUID
	Shell wsman.Shell
}

type runspaceClient interface {
	CreatePSRPShell(ctx context.Context, init psrp.RunspacePoolInit) (wsman.Shell, uuid.UUID, error)
	ReceiveShell(ctx context.Context, shell wsman.Shell, streams []string) (wsman.ReceiveResult, error)
	SendShell(ctx context.Context, shell wsman.Shell, stream string, input []byte, eof bool) error
}

func OpenRunspacePool(ctx context.Context, client runspaceClient, init psrp.RunspacePoolInit) (RunspacePool, []psrp.Message, error) {
	shell, runspacePoolID, err := client.CreatePSRPShell(ctx, init)
	if err != nil {
		return RunspacePool{}, nil, err
	}

	info, messages, err := WaitForRunspacePoolState(ctx, client, shell)
	if err != nil {
		return RunspacePool{}, nil, err
	}
	if info.State != psrp.RunspacePoolStateOpened && info.State != psrp.RunspacePoolStateNegotiationSucceeded {
		return RunspacePool{}, messages, fmt.Errorf("psrp runspace pool did not open: %s", info.StateName)
	}

	return RunspacePool{
		ID:    runspacePoolID,
		Shell: shell,
	}, messages, nil
}
