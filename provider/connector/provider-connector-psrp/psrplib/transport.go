package psrplib

import (
	"context"
	"fmt"

	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/psrp"
	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/wsman"
	"github.com/google/uuid"
)

type psrpTransportClient interface {
	SendShell(ctx context.Context, shell wsman.Shell, stream string, input []byte, eof bool) error
	ReceiveShell(ctx context.Context, shell wsman.Shell, streams []string) (wsman.ReceiveResult, error)
}

func SendPSRPMessages(ctx context.Context, client psrpTransportClient, shell wsman.Shell, stream string, messages []psrp.Message) error {
	objectID := uint64(1)
	for _, message := range messages {
		fragments := psrp.FragmentMessage(objectID, psrp.EncodeMessage(message), 0)
		payload := make([]byte, 0)
		for _, fragment := range fragments {
			payload = append(payload, psrp.EncodeFragment(fragment)...)
		}
		if err := client.SendShell(ctx, shell, stream, payload, false); err != nil {
			return err
		}
		objectID++
	}
	return nil
}

func ReceivePSRPMessages(ctx context.Context, client psrpTransportClient, shell wsman.Shell) ([]psrp.Message, error) {
	result, err := client.ReceiveShell(ctx, shell, []string{"stdout"})
	if err != nil {
		return nil, err
	}
	if len(result.Stdout) == 0 {
		return nil, nil
	}
	return psrp.DecodeMessages(result.Stdout)
}

func WaitForRunspacePoolState(ctx context.Context, client psrpTransportClient, shell wsman.Shell) (psrp.RunspacePoolStateInfo, []psrp.Message, error) {
	collected := []psrp.Message{}
	decoder := &psrp.MessageStreamDecoder{}
	for {
		result, err := client.ReceiveShell(ctx, shell, []string{"stdout"})
		if err != nil {
			return psrp.RunspacePoolStateInfo{}, collected, err
		}
		messages, err := decoder.Push(result.Stdout)
		if err != nil {
			return psrp.RunspacePoolStateInfo{}, collected, err
		}
		collected = append(collected, messages...)
		for _, message := range messages {
			if message.Type != psrp.MessageRunspacePoolState {
				continue
			}
			info, err := psrp.ParseRunspacePoolStateData(message.Data)
			if err != nil {
				return psrp.RunspacePoolStateInfo{}, collected, err
			}
			if info.State.Terminal() {
				return info, collected, nil
			}
		}
		if len(messages) == 0 {
			return psrp.RunspacePoolStateInfo{}, collected, fmt.Errorf("psrp bootstrap did not return a runspace pool state")
		}
	}
}

func WaitForPipelineState(ctx context.Context, client psrpTransportClient, shell wsman.Shell, pipelineID string) (psrp.PipelineStateInfo, []psrp.Message, error) {
	collected := []psrp.Message{}
	decoder := &psrp.MessageStreamDecoder{}
	for {
		result, err := client.ReceiveShell(ctx, shell, []string{"stdout"})
		if err != nil {
			return psrp.PipelineStateInfo{}, collected, err
		}
		messages, err := decoder.Push(result.Stdout)
		if err != nil {
			return psrp.PipelineStateInfo{}, collected, err
		}
		collected = append(collected, messages...)
		for _, message := range messages {
			if message.Type != psrp.MessagePipelineState {
				continue
			}
			if pipelineID != "" && message.PipelineID.String() != pipelineID {
				continue
			}
			info, err := psrp.ParsePipelineStateData(message.Data)
			if err != nil {
				return psrp.PipelineStateInfo{}, collected, err
			}
			if info.State.Terminal() {
				return info, collected, nil
			}
		}
		if len(messages) == 0 {
			return psrp.PipelineStateInfo{}, collected, fmt.Errorf("psrp pipeline did not return a terminal state")
		}
	}
}

func SendPipelineInput(ctx context.Context, client psrpTransportClient, shell wsman.Shell, runspacePoolID, pipelineID uuid.UUID, input string, eof bool) error {
	messages := []psrp.Message{
		psrp.BuildPipelineInputMessage(runspacePoolID, pipelineID, input),
	}
	if eof {
		messages = append(messages, psrp.BuildEndOfPipelineInputMessage(runspacePoolID, pipelineID))
	}
	return SendPSRPMessages(ctx, client, shell, "stdin", messages)
}

func SendPipelineHostResponse(ctx context.Context, client psrpTransportClient, shell wsman.Shell, runspacePoolID, pipelineID uuid.UUID, callID int, resultXML string, errorXML string) error {
	return SendPSRPMessages(ctx, client, shell, "stdin", []psrp.Message{
		psrp.BuildPipelineHostResponseMessage(runspacePoolID, pipelineID, callID, resultXML, errorXML),
	})
}
