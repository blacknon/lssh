package psrplib

import (
	"strings"
	"testing"
)

func TestBuildPromptResponseXML(t *testing.T) {
	raw := buildPromptResponseXML("name=alice, team=ops")
	for _, want := range []string{
		`<S N="Key">name</S><S N="Value">alice</S>`,
		`<S N="Key">team</S><S N="Value">ops</S>`,
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("buildPromptResponseXML() missing %q in %s", want, raw)
		}
	}
}

func TestBuildChoiceResponseXML(t *testing.T) {
	if got := buildChoiceResponseXML("2"); got != `<I32>2</I32>` {
		t.Fatalf("buildChoiceResponseXML() = %q", got)
	}
}

func TestBuildMultiChoiceResponseXML(t *testing.T) {
	raw := buildMultiChoiceResponseXML("1, 3")
	for _, want := range []string{
		`<I32>1</I32>`,
		`<I32>3</I32>`,
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("buildMultiChoiceResponseXML() missing %q in %s", want, raw)
		}
	}
}
