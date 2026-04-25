package psrp

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestBuildCreatePipelineData(t *testing.T) {
	raw := string(BuildCreatePipelineData(`'test'`, true))
	for _, want := range []string{
		`<S N="Cmd">&apos;test&apos;</S>`,
		`<B N="IsScript">true</B>`,
		`<B N="NoInput">true</B>`,
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("BuildCreatePipelineData() missing %q in %s", want, raw)
		}
	}
}

func TestBuildCreatePipelineMessage(t *testing.T) {
	runspacePoolID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pipelineID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	message := BuildCreatePipelineMessage(runspacePoolID, pipelineID, `'test'`, true)
	if message.Type != MessageCreatePipeline {
		t.Fatalf("Type = %v, want %v", message.Type, MessageCreatePipeline)
	}
	if message.RunspacePoolID != runspacePoolID {
		t.Fatalf("RunspacePoolID = %v, want %v", message.RunspacePoolID, runspacePoolID)
	}
	if message.PipelineID != pipelineID {
		t.Fatalf("PipelineID = %v, want %v", message.PipelineID, pipelineID)
	}
}

func TestParsePipelineStateData(t *testing.T) {
	raw := []byte(`<Obj RefId="0"><MS><Obj N="PipelineState" RefId="1"><ToString>Completed</ToString><I32>4</I32></Obj></MS></Obj>`)

	info, err := ParsePipelineStateData(raw)
	if err != nil {
		t.Fatalf("ParsePipelineStateData() error = %v", err)
	}
	if info.State != PipelineStateCompleted {
		t.Fatalf("State = %v, want %v", info.State, PipelineStateCompleted)
	}
	if !info.State.Terminal() {
		t.Fatal("Completed should be terminal")
	}
}
