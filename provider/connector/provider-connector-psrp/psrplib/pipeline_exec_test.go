package psrplib

import (
	"context"
	"testing"

	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/psrp"
	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/wsman"
	"github.com/google/uuid"
)

func TestStartPipeline(t *testing.T) {
	runspacePoolID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	client := &fakePipelineStarter{
		startPipelineFn: func(ctx context.Context, shell wsman.Shell, pipelineID uuid.UUID, message psrp.Message) (wsman.Command, error) {
			if message.Type != psrp.MessageCreatePipeline {
				t.Fatalf("message.Type = %v, want %v", message.Type, psrp.MessageCreatePipeline)
			}
			if message.RunspacePoolID != runspacePoolID {
				t.Fatalf("message.RunspacePoolID = %v, want %v", message.RunspacePoolID, runspacePoolID)
			}
			return wsman.Command{ID: "uuid:command-id"}, nil
		},
	}

	pipeline, err := StartPipeline(context.Background(), client, RunspacePool{
		ID:    runspacePoolID,
		Shell: wsman.Shell{ID: "uuid:shell-id"},
	}, `'test'`, true)
	if err != nil {
		t.Fatalf("StartPipeline() error = %v", err)
	}
	if pipeline.Command.ID != "uuid:command-id" {
		t.Fatalf("pipeline.Command.ID = %q, want uuid:command-id", pipeline.Command.ID)
	}
}

func TestWaitForPipelineCompletion(t *testing.T) {
	runspacePoolID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pipelineID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	payload := psrp.EncodeFragment(psrp.Fragment{
		ObjectID:   1,
		FragmentID: 0,
		Start:      true,
		End:        true,
		Blob: psrp.EncodeMessage(psrp.Message{
			Destination:    psrp.DestinationClient,
			Type:           psrp.MessagePipelineState,
			RunspacePoolID: runspacePoolID,
			PipelineID:     pipelineID,
			Data:           []byte(`<Obj RefId="0"><MS><Obj N="PipelineState" RefId="1"><ToString>Completed</ToString><I32>4</I32></Obj></MS></Obj>`),
		}),
	})

	client := &fakePipelineStarter{
		receiveFn: func(ctx context.Context, shell wsman.Shell, command wsman.Command) (wsman.ReceiveResult, error) {
			return wsman.ReceiveResult{Stdout: payload, Done: true}, nil
		},
	}

	result, err := WaitForPipelineCompletion(context.Background(), client, wsman.Shell{ID: "uuid:shell-id"}, Pipeline{
		ID:      pipelineID,
		Command: wsman.Command{ID: "uuid:command-id"},
	})
	if err != nil {
		t.Fatalf("WaitForPipelineCompletion() error = %v", err)
	}
	if result.State.State != psrp.PipelineStateCompleted {
		t.Fatalf("result.State.State = %v, want %v", result.State.State, psrp.PipelineStateCompleted)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("len(result.Messages) = %d, want 1", len(result.Messages))
	}
}

type fakePipelineStarter struct {
	startPipelineFn func(ctx context.Context, shell wsman.Shell, pipelineID uuid.UUID, message psrp.Message) (wsman.Command, error)
	receiveFn       func(ctx context.Context, shell wsman.Shell, command wsman.Command) (wsman.ReceiveResult, error)
}

func (f *fakePipelineStarter) StartPipeline(ctx context.Context, shell wsman.Shell, pipelineID uuid.UUID, message psrp.Message) (wsman.Command, error) {
	return f.startPipelineFn(ctx, shell, pipelineID, message)
}

func (f *fakePipelineStarter) Receive(ctx context.Context, shell wsman.Shell, command wsman.Command) (wsman.ReceiveResult, error) {
	return f.receiveFn(ctx, shell, command)
}
