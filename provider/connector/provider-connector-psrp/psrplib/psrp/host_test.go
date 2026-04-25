package psrp

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestParseHostCallData(t *testing.T) {
	raw := []byte(
		`<Obj RefId="0"><MS>` +
			`<I64 N="ci">12</I64>` +
			`<Obj N="mi" RefId="1"><ToString>WriteLine2</ToString></Obj>` +
			`<Obj N="mp" RefId="2"><LST><S>hello</S><S>world</S></LST></Obj>` +
			`</MS></Obj>`,
	)

	call, err := ParseHostCallData(raw)
	if err != nil {
		t.Fatalf("ParseHostCallData() error = %v", err)
	}
	if call.CallID != 12 {
		t.Fatalf("CallID = %d, want 12", call.CallID)
	}
	if call.MethodIdentifier != "WriteLine2" {
		t.Fatalf("MethodIdentifier = %q, want WriteLine2", call.MethodIdentifier)
	}
	if len(call.Parameters) != 2 {
		t.Fatalf("len(Parameters) = %d, want 2", len(call.Parameters))
	}
}

func TestBuildPipelineHostResponseMessage(t *testing.T) {
	message := BuildPipelineHostResponseMessage(
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		7,
		"",
		"",
	)
	if message.Type != MessagePipelineHostResponse {
		t.Fatalf("Type = %v, want %v", message.Type, MessagePipelineHostResponse)
	}
	if !strings.Contains(string(message.Data), `<I64 N="ci">7</I64>`) {
		t.Fatalf("response missing call id: %s", message.Data)
	}
}

func TestBuildDefaultHostResponse(t *testing.T) {
	runspacePoolID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pipelineID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	message := BuildDefaultHostResponse(runspacePoolID, pipelineID, MessagePipelineHostCall, HostCall{
		CallID:     9,
		MethodName: "read_line",
	})
	if message.Type != MessagePipelineHostResponse {
		t.Fatalf("Type = %v, want %v", message.Type, MessagePipelineHostResponse)
	}
	if !strings.Contains(string(message.Data), `<S></S>`) {
		t.Fatalf("response missing string result: %s", message.Data)
	}
}

func TestFormatHostPrompt(t *testing.T) {
	got := FormatHostPrompt(HostCall{
		MethodName: "prompt_for_choice",
		Parameters: []string{"A", "B"},
	})
	if got != "choose one: A | B" {
		t.Fatalf("FormatHostPrompt() = %q", got)
	}
}

func TestHostMethodNameAliases(t *testing.T) {
	cases := map[string]string{
		"PromptForChoice":                  "prompt_for_choice",
		"PromptForChoiceMultipleSelection": "prompt_for_choice_multiple_selection",
		"ReadLineAsSecureString":           "read_line",
		"WriteProgress":                    "write_progress",
	}

	for input, want := range cases {
		if got := hostMethodName(input); got != want {
			t.Fatalf("hostMethodName(%q) = %q, want %q", input, got, want)
		}
	}
}
