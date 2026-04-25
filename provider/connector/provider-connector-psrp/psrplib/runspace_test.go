package psrplib

import (
	"context"
	"fmt"
	"testing"

	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/psrp"
	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/wsman"
	"github.com/google/uuid"
)

func TestOpenRunspacePool(t *testing.T) {
	runspacePoolID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	serverMessage := psrp.EncodeMessage(psrp.Message{
		Destination:    psrp.DestinationClient,
		Type:           psrp.MessageRunspacePoolState,
		RunspacePoolID: runspacePoolID,
		PipelineID:     uuid.Nil,
		Data:           []byte(`<Obj RefId="0"><MS><Obj N="RunspaceState" RefId="1"><ToString>Opened</ToString><I32>2</I32></Obj></MS></Obj>`),
	})
	streamPayload := psrp.EncodeFragment(psrp.Fragment{
		ObjectID:   10,
		FragmentID: 0,
		Start:      true,
		End:        true,
		Blob:       serverMessage,
	})

	client := fakeWSManClient{
		createPSRPShellFn: func(ctx context.Context, init psrp.RunspacePoolInit) (wsman.Shell, uuid.UUID, error) {
			return wsman.Shell{
				ID:          "uuid:shell-id",
				ResourceURI: "http://schemas.microsoft.com/powershell/Microsoft.PowerShell",
			}, runspacePoolID, nil
		},
		receiveShellFn: func(ctx context.Context, shell wsman.Shell, streams []string) (wsman.ReceiveResult, error) {
			return wsman.ReceiveResult{Stdout: streamPayload}, nil
		},
	}

	pool, messages, err := OpenRunspacePool(context.Background(), client, psrp.DefaultRunspacePoolInit())
	if err != nil {
		t.Fatalf("OpenRunspacePool() error = %v", err)
	}
	if pool.ID != runspacePoolID {
		t.Fatalf("RunspacePool.ID = %v, want %v", pool.ID, runspacePoolID)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].Type != psrp.MessageRunspacePoolState {
		t.Fatalf("messages[0].Type = %v, want %v", messages[0].Type, psrp.MessageRunspacePoolState)
	}
}

type fakeWSManClient struct {
	createPSRPShellFn func(ctx context.Context, init psrp.RunspacePoolInit) (wsman.Shell, uuid.UUID, error)
	receiveShellFn    func(ctx context.Context, shell wsman.Shell, streams []string) (wsman.ReceiveResult, error)
}

func (f fakeWSManClient) Do(ctx context.Context, header wsman.Header, body string) ([]byte, error) {
	return nil, fmt.Errorf("unexpected Do call")
}

func (f fakeWSManClient) CreateShell(ctx context.Context, powershell bool) (wsman.Shell, error) {
	return wsman.Shell{}, fmt.Errorf("unexpected CreateShell call")
}

func (f fakeWSManClient) CreatePSRPShell(ctx context.Context, init psrp.RunspacePoolInit) (wsman.Shell, uuid.UUID, error) {
	return f.createPSRPShellFn(ctx, init)
}

func (f fakeWSManClient) DeleteShell(ctx context.Context, shell wsman.Shell) error {
	return nil
}

func (f fakeWSManClient) Execute(ctx context.Context, shell wsman.Shell, command string, arguments ...string) (wsman.Command, error) {
	return wsman.Command{}, fmt.Errorf("unexpected Execute call")
}

func (f fakeWSManClient) Receive(ctx context.Context, shell wsman.Shell, command wsman.Command) (wsman.ReceiveResult, error) {
	return wsman.ReceiveResult{}, fmt.Errorf("unexpected Receive call")
}

func (f fakeWSManClient) ReceiveShell(ctx context.Context, shell wsman.Shell, streams []string) (wsman.ReceiveResult, error) {
	return f.receiveShellFn(ctx, shell, streams)
}

func (f fakeWSManClient) Send(ctx context.Context, shell wsman.Shell, command wsman.Command, input []byte, eof bool) error {
	return fmt.Errorf("unexpected Send call")
}

func (f fakeWSManClient) SendShell(ctx context.Context, shell wsman.Shell, stream string, input []byte, eof bool) error {
	return fmt.Errorf("unexpected SendShell call")
}

func (f fakeWSManClient) SignalTerminate(ctx context.Context, shell wsman.Shell, command wsman.Command) error {
	return fmt.Errorf("unexpected SignalTerminate call")
}
