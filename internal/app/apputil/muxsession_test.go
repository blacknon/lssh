package apputil

import (
	"reflect"
	"testing"
)

func TestBuildPersistentSessionArgs(t *testing.T) {
	got := BuildPersistentSessionArgs(PersistentSessionArgsConfig{
		AllArgs: []string{
			"--detach",
			"--session", "ops",
			"--socket-path", "/tmp/ops.sock",
			"--host", "web01",
		},
		BareFlags: map[string]bool{
			"--detach": true,
		},
		ValueFlags: map[string]bool{
			"--session":     true,
			"--socket-path": true,
		},
		DaemonFlag:  "--mux-daemon",
		SessionFlag: "--session",
		SocketFlag:  "--socket-path",
		Name:        "prod",
		SocketPath:  "/tmp/prod.sock",
	})

	want := []string{"--host", "web01", "--mux-daemon", "--session", "prod", "--socket-path", "/tmp/prod.sock"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildPersistentSessionArgs() = %#v, want %#v", got, want)
	}
}
