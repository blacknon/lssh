package ssh

import (
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
)

func TestGetProxyRouteUsesHopPorts(t *testing.T) {
	t.Parallel()

	cfg := conf.Config{
		Server: map[string]conf.ServerConfig{
			"ssh_proxy": {
				Addr: "172.31.0.31",
				Port: "22",
			},
			"OverSshProxy": {
				Addr:  "172.31.1.41",
				Port:  "22",
				Proxy: "ssh_proxy",
			},
			"OverNestedHttpProxy": {
				Addr:      "172.31.3.71",
				Port:      "22",
				Proxy:     "deep_http_proxy",
				ProxyType: "http",
			},
		},
		Proxy: map[string]conf.ProxyConfig{
			"deep_http_proxy": {
				Addr:  "172.31.2.61",
				Port:  "8888",
				Proxy: "OverSshProxy",
			},
		},
	}

	route, err := getProxyRoute("OverNestedHttpProxy", cfg)
	if err != nil {
		t.Fatalf("getProxyRoute returned error: %v", err)
	}

	if len(route) != 3 {
		t.Fatalf("getProxyRoute returned %d hops, want 3", len(route))
	}

	if route[0].Type != "ssh" || route[0].Name != "ssh_proxy" || route[0].Port != "22" {
		t.Fatalf("route[0] = %#v, want ssh hop to ssh_proxy:22", route[0])
	}

	if route[1].Type != "ssh" || route[1].Name != "OverSshProxy" || route[1].Port != "22" {
		t.Fatalf("route[1] = %#v, want ssh hop to OverSshProxy:22", route[1])
	}

	if route[2].Type != "http" || route[2].Name != "deep_http_proxy" || route[2].Port != "8888" {
		t.Fatalf("route[2] = %#v, want http hop to deep_http_proxy:8888", route[2])
	}
}
