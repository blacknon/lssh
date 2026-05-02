package lssh

import (
	"reflect"
	"strings"
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

func TestBuildMuxDaemonArgs(t *testing.T) {
	args := []string{
		"-P",
		"--mux-session", "ops",
		"--mux-socket-path", "/tmp/ops.sock",
		"--mux-detach",
		"--host", "web01",
	}

	got := buildMuxDaemonArgs(args, "prod", "/tmp/prod.sock")
	want := []string{"-P", "--host", "web01", "--mux-daemon", "--mux-session", "prod", "--mux-socket-path", "/tmp/prod.sock"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildMuxDaemonArgs = %#v, want %#v", got, want)
	}
}

func TestBuildBackgroundArgs(t *testing.T) {
	args := []string{"-f", "--file", "conf.toml", "--parallel"}

	got := buildBackgroundArgs(args, []string{"web01", "db01"}, nil)
	want := []string{"--file", "conf.toml", "--parallel", "-H", "web01", "-H", "db01"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildBackgroundArgs = %#v, want %#v", got, want)
	}
}

func TestResolveTunnelOption(t *testing.T) {
	tests := []struct {
		name        string
		goos        string
		spec        string
		wantEnabled bool
		wantLocal   int
		wantRemote  int
		wantErr     string
	}{
		{
			name:        "empty",
			goos:        "linux",
			spec:        "",
			wantEnabled: false,
		},
		{
			name:        "linux valid",
			goos:        "linux",
			spec:        "1:any",
			wantEnabled: true,
			wantLocal:   1,
			wantRemote:  -1,
		},
		{
			name:    "windows unsupported",
			goos:    "windows",
			spec:    "0:0",
			wantErr: "not supported on Windows",
		},
		{
			name:    "invalid format",
			goos:    "linux",
			spec:    "bad",
			wantErr: "invalid --tunnel format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEnabled, gotLocal, gotRemote, err := resolveTunnelOption(tt.goos, tt.spec)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("resolveTunnelOption(%q, %q) error = nil", tt.goos, tt.spec)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveTunnelOption(%q, %q) error = %q, want substring %q", tt.goos, tt.spec, err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveTunnelOption(%q, %q) error = %v", tt.goos, tt.spec, err)
			}
			if gotEnabled != tt.wantEnabled || gotLocal != tt.wantLocal || gotRemote != tt.wantRemote {
				t.Fatalf(
					"resolveTunnelOption(%q, %q) = (%t, %d, %d), want (%t, %d, %d)",
					tt.goos,
					tt.spec,
					gotEnabled,
					gotLocal,
					gotRemote,
					tt.wantEnabled,
					tt.wantLocal,
					tt.wantRemote,
				)
			}
		})
	}
}
