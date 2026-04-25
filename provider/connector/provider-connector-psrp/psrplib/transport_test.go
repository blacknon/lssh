package psrplib

import (
	"context"
	"testing"

	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/psrp"
	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/wsman"
	"github.com/google/uuid"
)

func TestSendPSRPMessages(t *testing.T) {
	client := &fakeTransportClient{}
	shell := wsman.Shell{ID: "uuid:shell-id"}

	err := SendPSRPMessages(context.Background(), client, shell, "stdin", []psrp.Message{{
		Destination:    psrp.DestinationServer,
		Type:           psrp.MessageSessionCapability,
		RunspacePoolID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		PipelineID:     uuid.Nil,
		Data:           []byte("payload"),
	}})
	if err != nil {
		t.Fatalf("SendPSRPMessages() error = %v", err)
	}
	if len(client.sent) != 1 {
		t.Fatalf("len(sent) = %d, want 1", len(client.sent))
	}
	if client.sent[0].stream != "stdin" {
		t.Fatalf("stream = %q, want stdin", client.sent[0].stream)
	}
}

func TestWaitForRunspacePoolState(t *testing.T) {
	runspacePoolID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	stateMessage := psrp.EncodeMessage(psrp.Message{
		Destination:    psrp.DestinationClient,
		Type:           psrp.MessageRunspacePoolState,
		RunspacePoolID: runspacePoolID,
		PipelineID:     uuid.Nil,
		Data:           []byte(`<Obj RefId="0"><MS><Obj N="RunspaceState" RefId="1"><ToString>Opened</ToString><I32>2</I32></Obj></MS></Obj>`),
	})
	client := &fakeTransportClient{
		receivePayloads: [][]byte{
			psrp.EncodeFragment(psrp.Fragment{
				ObjectID:   1,
				FragmentID: 0,
				Start:      true,
				End:        true,
				Blob:       stateMessage,
			}),
		},
	}

	info, messages, err := WaitForRunspacePoolState(context.Background(), client, wsman.Shell{ID: "uuid:shell-id"})
	if err != nil {
		t.Fatalf("WaitForRunspacePoolState() error = %v", err)
	}
	if info.State != psrp.RunspacePoolStateOpened {
		t.Fatalf("State = %v, want %v", info.State, psrp.RunspacePoolStateOpened)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
}

func TestWaitForPipelineState(t *testing.T) {
	runspacePoolID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pipelineID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	stateMessage := psrp.EncodeMessage(psrp.Message{
		Destination:    psrp.DestinationClient,
		Type:           psrp.MessagePipelineState,
		RunspacePoolID: runspacePoolID,
		PipelineID:     pipelineID,
		Data:           []byte(`<Obj RefId="0"><MS><Obj N="PipelineState" RefId="1"><ToString>Completed</ToString><I32>4</I32></Obj></MS></Obj>`),
	})
	client := &fakeTransportClient{
		receivePayloads: [][]byte{
			psrp.EncodeFragment(psrp.Fragment{
				ObjectID:   1,
				FragmentID: 0,
				Start:      true,
				End:        true,
				Blob:       stateMessage,
			}),
		},
	}

	info, messages, err := WaitForPipelineState(context.Background(), client, wsman.Shell{ID: "uuid:shell-id"}, pipelineID.String())
	if err != nil {
		t.Fatalf("WaitForPipelineState() error = %v", err)
	}
	if info.State != psrp.PipelineStateCompleted {
		t.Fatalf("State = %v, want %v", info.State, psrp.PipelineStateCompleted)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
}

func TestSendPipelineInput(t *testing.T) {
	runspacePoolID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pipelineID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	client := &fakeTransportClient{}

	err := SendPipelineInput(context.Background(), client, wsman.Shell{ID: "uuid:shell-id"}, runspacePoolID, pipelineID, "hello", true)
	if err != nil {
		t.Fatalf("SendPipelineInput() error = %v", err)
	}
	if len(client.sent) != 2 {
		t.Fatalf("len(sent) = %d, want 2", len(client.sent))
	}
	for _, payload := range client.sent {
		if payload.stream != "stdin" {
			t.Fatalf("stream = %q, want stdin", payload.stream)
		}
	}
}

type fakeTransportClient struct {
	sent            []sentPayload
	receivePayloads [][]byte
}

type sentPayload struct {
	stream string
	input  []byte
	eof    bool
}

func (f *fakeTransportClient) SendShell(ctx context.Context, shell wsman.Shell, stream string, input []byte, eof bool) error {
	f.sent = append(f.sent, sentPayload{
		stream: stream,
		input:  append([]byte(nil), input...),
		eof:    eof,
	})
	return nil
}

func (f *fakeTransportClient) ReceiveShell(ctx context.Context, shell wsman.Shell, streams []string) (wsman.ReceiveResult, error) {
	if len(f.receivePayloads) == 0 {
		return wsman.ReceiveResult{}, nil
	}
	payload := f.receivePayloads[0]
	f.receivePayloads = f.receivePayloads[1:]
	return wsman.ReceiveResult{Stdout: payload}, nil
}
