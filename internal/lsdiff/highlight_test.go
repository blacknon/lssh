package lsdiff

import (
	"strings"
	"testing"
)

func TestRenderStyledLineHighlightsInlineDiff(t *testing.T) {
	got := renderStyledLine("/tmp/test.go", "return changed", "return value", "", true)
	if !strings.Contains(got, "maroon") {
		t.Fatalf("renderStyledLine() missing inline diff highlight: %q", got)
	}
}

func TestRenderStyledLineHighlightsSyntaxAndSearch(t *testing.T) {
	got := renderStyledLine("/tmp/test.go", `return "hello"`, "", "hello", false)
	if !strings.Contains(got, "deepskyblue") {
		t.Fatalf("renderStyledLine() missing keyword highlight: %q", got)
	}
	if !strings.Contains(got, "yellow") {
		t.Fatalf("renderStyledLine() missing search highlight: %q", got)
	}
}
