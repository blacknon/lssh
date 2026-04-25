package psrplib

import (
	"context"

	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/psrp"
	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/wsman"
	"github.com/google/uuid"
)

type Pipeline struct {
	ID      uuid.UUID
	Command wsman.Command
}

type pipelineStarter interface {
	StartPipeline(ctx context.Context, shell wsman.Shell, pipelineID uuid.UUID, message psrp.Message) (wsman.Command, error)
	Receive(ctx context.Context, shell wsman.Shell, command wsman.Command) (wsman.ReceiveResult, error)
}

type PipelineResult struct {
	Messages  []psrp.Message
	State     psrp.PipelineStateInfo
	CommandID string
	RawStdout [][]byte
	RawStderr [][]byte
	Outputs   [][]byte
	Errors    [][]byte
}

func StartPipeline(ctx context.Context, client pipelineStarter, pool RunspacePool, script string, noInput bool) (Pipeline, error) {
	pipelineID := uuid.New()
	command, err := client.StartPipeline(ctx, pool.Shell, pipelineID, psrp.BuildCreatePipelineMessage(pool.ID, pipelineID, script, noInput))
	if err != nil {
		return Pipeline{}, err
	}

	return Pipeline{
		ID:      pipelineID,
		Command: command,
	}, nil
}

func WaitForPipelineCompletion(ctx context.Context, client pipelineStarter, shell wsman.Shell, pipeline Pipeline) (PipelineResult, error) {
	result := PipelineResult{
		CommandID: pipeline.Command.ID,
	}
	decoder := &psrp.MessageStreamDecoder{}

	for {
		received, err := client.Receive(ctx, shell, pipeline.Command)
		if err != nil {
			return PipelineResult{}, err
		}
		if len(received.Stdout) > 0 {
			result.RawStdout = append(result.RawStdout, append([]byte(nil), received.Stdout...))
			messages, err := decoder.Push(received.Stdout)
			if err != nil {
				return PipelineResult{}, err
			}
			result.Messages = append(result.Messages, messages...)
			for _, message := range messages {
				if message.PipelineID != pipeline.ID {
					continue
				}
				switch message.Type {
				case psrp.MessagePipelineOutput:
					result.Outputs = append(result.Outputs, append([]byte(nil), message.Data...))
				case psrp.MessageErrorRecord:
					result.Errors = append(result.Errors, append([]byte(nil), message.Data...))
				case psrp.MessagePipelineState:
					state, err := psrp.ParsePipelineStateData(message.Data)
					if err != nil {
						return PipelineResult{}, err
					}
					result.State = state
					if state.State.Terminal() {
						return result, nil
					}
				}
			}
		}
		if len(received.Stderr) > 0 {
			result.RawStderr = append(result.RawStderr, append([]byte(nil), received.Stderr...))
		}
		if received.Done {
			return result, nil
		}
	}
}
