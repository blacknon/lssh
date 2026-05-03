package apputil

import (
	"reflect"
	"testing"
)

func TestFilterCLIArgs(t *testing.T) {
	args := []string{
		"--detach",
		"--session", "ops",
		"--socket-path", "/tmp/ops.sock",
		"--host", "web01",
		"command",
	}

	got := FilterCLIArgs(args, map[string]bool{"--detach": true}, map[string]bool{
		"--session":     true,
		"--socket-path": true,
	})
	want := []string{"--host", "web01", "command"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterCLIArgs() = %#v, want %#v", got, want)
	}
}
