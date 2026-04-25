package mux

import (
	"strings"
	"testing"
)

func TestBuildLocalRcCommandExportsTERM(t *testing.T) {
	t.Setenv("TERM", "screen-256color")

	cmd := buildLocalRcCommand([]string{}, "", false, "")
	if !strings.Contains(cmd, "export TERM=screen-256color;") {
		t.Fatalf("buildLocalRcCommand() = %q, want TERM export", cmd)
	}
}

func TestResizeDeduperShouldSend(t *testing.T) {
	var d resizeDeduper

	if !d.ShouldSend(128, 27) {
		t.Fatal("first resize was dropped, want sent")
	}
	if d.ShouldSend(128, 27) {
		t.Fatal("duplicate resize was sent, want dropped")
	}
	if !d.ShouldSend(130, 32) {
		t.Fatal("changed resize was dropped, want sent")
	}
}

func TestDedupeResizeFunc(t *testing.T) {
	var calls [][2]int

	resize := dedupeResizeFunc(80, 24, func(cols, rows int) error {
		calls = append(calls, [2]int{cols, rows})
		return nil
	})

	if err := resize(80, 24); err != nil {
		t.Fatalf("resize(80,24) error = %v", err)
	}
	if err := resize(80, 24); err != nil {
		t.Fatalf("resize(80,24) duplicate error = %v", err)
	}
	if err := resize(100, 30); err != nil {
		t.Fatalf("resize(100,30) error = %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("resize call count = %d, want 1 changed-size call only", len(calls))
	}
	if calls[0] != ([2]int{100, 30}) {
		t.Fatalf("resize calls[0] = %v, want [100 30]", calls[0])
	}
}
