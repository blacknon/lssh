package termenv

import (
	"strings"
	"testing"
)

func TestMergeEnvAddsTERMWhenMissing(t *testing.T) {
	t.Setenv("TERM", "")

	env := MergeEnv(nil)
	found := false
	for _, entry := range env {
		if strings.HasPrefix(entry, "TERM=") {
			found = true
			if entry != "TERM="+DefaultTerm {
				t.Fatalf("TERM entry = %q, want %q", entry, "TERM="+DefaultTerm)
			}
		}
	}
	if !found {
		t.Fatal("TERM entry missing after MergeEnv(nil)")
	}
}

func TestWrapShellExecExportsTERM(t *testing.T) {
	t.Setenv("TERM", "screen-256color")

	got := WrapShellExec("printf ok")
	if !strings.Contains(got, "export TERM='screen-256color';") {
		t.Fatalf("WrapShellExec() = %q, want TERM export", got)
	}
	if !strings.Contains(got, "exec printf ok") {
		t.Fatalf("WrapShellExec() = %q, want exec prefix", got)
	}
}
