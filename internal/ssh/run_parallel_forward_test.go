package ssh

import (
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
)

func TestPrepareParallelForwardConfigKeepsOnlyReverseSafeForwards(t *testing.T) {
	r := &Run{
		Conf: conf.Config{
			Server: map[string]conf.ServerConfig{
				"target": {
					PortForwardMode:               "local",
					PortForwardLocal:              "127.0.0.1:8080",
					PortForwardRemote:             "localhost:80",
					PortForwards:                  []string{"remote:10080:localhost:80", "local:18080:localhost:8080"},
					DynamicPortForward:            "1080",
					HTTPDynamicPortForward:        "18080",
					ReverseDynamicPortForward:     "2080",
					HTTPReverseDynamicPortForward: "28080",
					NFSDynamicForwardPort:         "2049",
					NFSDynamicForwardPath:         "/remote",
					NFSReverseDynamicForwardPort:  "3049",
					NFSReverseDynamicForwardPath:  "/local",
				},
			},
		},
	}

	config := r.PrepareParallelForwardConfig("target")

	if len(config.Forwards) != 1 {
		t.Fatalf("len(config.Forwards) = %d, want 1", len(config.Forwards))
	}
	if config.Forwards[0].Mode != "R" {
		t.Fatalf("config.Forwards[0].Mode = %q, want R", config.Forwards[0].Mode)
	}
	if config.Forwards[0].Local != "localhost:10080" {
		t.Fatalf("config.Forwards[0].Local = %q, want localhost:10080", config.Forwards[0].Local)
	}
	if config.Forwards[0].Remote != "localhost:80" {
		t.Fatalf("config.Forwards[0].Remote = %q, want localhost:80", config.Forwards[0].Remote)
	}

	if config.ReverseDynamicPortForward != "2080" {
		t.Fatalf("config.ReverseDynamicPortForward = %q, want 2080", config.ReverseDynamicPortForward)
	}
	if config.HTTPReverseDynamicPortForward != "28080" {
		t.Fatalf("config.HTTPReverseDynamicPortForward = %q, want 28080", config.HTTPReverseDynamicPortForward)
	}
	if config.NFSReverseDynamicForwardPort != "3049" {
		t.Fatalf("config.NFSReverseDynamicForwardPort = %q, want 3049", config.NFSReverseDynamicForwardPort)
	}
	if config.NFSReverseDynamicForwardPath != "/local" {
		t.Fatalf("config.NFSReverseDynamicForwardPath = %q, want /local", config.NFSReverseDynamicForwardPath)
	}

	if config.DynamicPortForward != "" {
		t.Fatalf("config.DynamicPortForward = %q, want empty", config.DynamicPortForward)
	}
	if config.HTTPDynamicPortForward != "" {
		t.Fatalf("config.HTTPDynamicPortForward = %q, want empty", config.HTTPDynamicPortForward)
	}
	if config.NFSDynamicForwardPort != "" || config.NFSDynamicForwardPath != "" {
		t.Fatalf("NFS dynamic forward was not cleared: %q %q", config.NFSDynamicForwardPort, config.NFSDynamicForwardPath)
	}
}

func TestPrepareParallelForwardConfigUsesRunOverrides(t *testing.T) {
	r := &Run{
		Conf: conf.Config{
			Server: map[string]conf.ServerConfig{
				"target": {
					ReverseDynamicPortForward:     "2080",
					HTTPReverseDynamicPortForward: "28080",
					NFSReverseDynamicForwardPort:  "3049",
					NFSReverseDynamicForwardPath:  "/config-local",
				},
			},
		},
		ReverseDynamicPortForward:     "3080",
		HTTPReverseDynamicPortForward: "38080",
		NFSReverseDynamicForwardPort:  "4049",
		NFSReverseDynamicForwardPath:  "/override-local",
		PortForward: []*conf.PortForward{
			{
				Mode:   "R",
				Local:  "localhost:40080",
				Remote: "localhost:80",
			},
		},
	}

	config := r.PrepareParallelForwardConfig("target")

	if config.ReverseDynamicPortForward != "3080" {
		t.Fatalf("config.ReverseDynamicPortForward = %q, want 3080", config.ReverseDynamicPortForward)
	}
	if config.HTTPReverseDynamicPortForward != "38080" {
		t.Fatalf("config.HTTPReverseDynamicPortForward = %q, want 38080", config.HTTPReverseDynamicPortForward)
	}
	if config.NFSReverseDynamicForwardPort != "4049" {
		t.Fatalf("config.NFSReverseDynamicForwardPort = %q, want 4049", config.NFSReverseDynamicForwardPort)
	}
	if config.NFSReverseDynamicForwardPath != "/override-local" {
		t.Fatalf("config.NFSReverseDynamicForwardPath = %q, want /override-local", config.NFSReverseDynamicForwardPath)
	}
	if len(config.Forwards) != 1 {
		t.Fatalf("len(config.Forwards) = %d, want 1", len(config.Forwards))
	}
	if config.Forwards[0].Local != "localhost:40080" || config.Forwards[0].Remote != "localhost:80" {
		t.Fatalf("config.Forwards[0] = %#v, want CLI remote forward", config.Forwards[0])
	}
}

func TestParallelIgnoredFeatures(t *testing.T) {
	r := &Run{
		Conf: conf.Config{
			Server: map[string]conf.ServerConfig{
				"target": {
					PortForwards:           []string{"local:18080:localhost:8080", "remote:10080:localhost:80"},
					DynamicPortForward:     "1080",
					HTTPDynamicPortForward: "18080",
					NFSDynamicForwardPort:  "2049",
					NFSDynamicForwardPath:  "/remote",
				},
			},
		},
		TunnelEnabled: true,
		TunnelLocal:   0,
		TunnelRemote:  1,
	}

	got := r.ParallelIgnoredFeatures("target")
	if len(got) != 5 {
		t.Fatalf("len(ParallelIgnoredFeatures()) = %d, want 5: %#v", len(got), got)
	}
}
