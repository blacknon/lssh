package psrp

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestBuildPipelineInputData(t *testing.T) {
	raw := string(BuildPipelineInputData(`<test>&value`))
	if !strings.Contains(raw, `<S>&lt;test&gt;&amp;value</S>`) {
		t.Fatalf("BuildPipelineInputData() = %s", raw)
	}
}

func TestBuildPipelineInputMessage(t *testing.T) {
	runspacePoolID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pipelineID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	message := BuildPipelineInputMessage(runspacePoolID, pipelineID, "hello")
	if message.Type != MessagePipelineInput {
		t.Fatalf("Type = %v, want %v", message.Type, MessagePipelineInput)
	}
	if message.RunspacePoolID != runspacePoolID {
		t.Fatalf("RunspacePoolID = %v, want %v", message.RunspacePoolID, runspacePoolID)
	}
	if message.PipelineID != pipelineID {
		t.Fatalf("PipelineID = %v, want %v", message.PipelineID, pipelineID)
	}
}

func TestBuildEndOfPipelineInputMessage(t *testing.T) {
	runspacePoolID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pipelineID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	message := BuildEndOfPipelineInputMessage(runspacePoolID, pipelineID)
	if message.Type != MessageEndOfPipelineInput {
		t.Fatalf("Type = %v, want %v", message.Type, MessageEndOfPipelineInput)
	}
	if len(message.Data) != 0 {
		t.Fatalf("Data length = %d, want 0", len(message.Data))
	}
}
