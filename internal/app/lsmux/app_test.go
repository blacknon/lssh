package lsmux

import (
	"reflect"
	"testing"
)

func TestFilterMuxSessionValueFlags(t *testing.T) {
	args := []string{
		"--session", "ops",
		"--socket-path", "/tmp/ops.sock",
		"--detach",
		"--host", "web01",
	}

	got := filterMuxSessionValueFlags(args)
	want := []string{"--detach", "--host", "web01"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filterMuxSessionValueFlags = %#v, want %#v", got, want)
	}
}
