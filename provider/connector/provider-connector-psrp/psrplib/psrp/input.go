package psrp

import (
	"fmt"

	"github.com/google/uuid"
)

func BuildPipelineInputData(input string) []byte {
	return []byte(fmt.Sprintf(`<Obj RefId="0"><S>%s</S></Obj>`, xmlEscape(input)))
}

func BuildPipelineInputMessage(runspacePoolID, pipelineID uuid.UUID, input string) Message {
	return Message{
		Destination:    DestinationServer,
		Type:           MessagePipelineInput,
		RunspacePoolID: runspacePoolID,
		PipelineID:     pipelineID,
		Data:           BuildPipelineInputData(input),
	}
}

func BuildEndOfPipelineInputMessage(runspacePoolID, pipelineID uuid.UUID) Message {
	return Message{
		Destination:    DestinationServer,
		Type:           MessageEndOfPipelineInput,
		RunspacePoolID: runspacePoolID,
		PipelineID:     pipelineID,
	}
}
