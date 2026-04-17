package lssh

import (
	"reflect"
	"testing"
)

func TestFilterLsshMuxSessionValueFlags(t *testing.T) {
	args := []string{
		"-P",
		"--mux-session", "ops",
		"--mux-socket-path", "/tmp/ops.sock",
		"--mux-detach",
		"--host", "web01",
	}

	got := filterLsshMuxSessionValueFlags(args)
	want := []string{"-P", "--mux-detach", "--host", "web01"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filterLsshMuxSessionValueFlags = %#v, want %#v", got, want)
	}
}
