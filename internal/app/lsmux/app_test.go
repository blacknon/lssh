package lsmux

import (
	"reflect"
	"testing"
)

func TestTranslateCompatArgs(t *testing.T) {
	args := []string{
		"lsmux",
		"--session", "ops",
		"--socket-path", "/tmp/ops.sock",
		"--detach",
		"--attach",
		"--list-sessions",
		"--kill-session",
		"--host", "web01",
	}

	got := TranslateCompatArgs(args)
	want := []string{
		"lsmux",
		"-P",
		"--mux-session", "ops",
		"--mux-socket-path", "/tmp/ops.sock",
		"--mux-detach",
		"--mux-attach",
		"--mux-list-sessions",
		"--mux-kill-session",
		"--host", "web01",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TranslateCompatArgs() = %#v, want %#v", got, want)
	}
}

func TestTranslateCompatArgsSupportsEqualsSyntax(t *testing.T) {
	got := TranslateCompatArgs([]string{"lsmux", "--session=ops", "--socket-path=/tmp/ops.sock"})
	want := []string{"lsmux", "-P", "--mux-session=ops", "--mux-socket-path=/tmp/ops.sock"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TranslateCompatArgs() = %#v, want %#v", got, want)
	}
}
