package conf

import (
	"net/netip"
	"testing"
)

func TestResolveConditionalMatchesIgnoreFalseOverrideAndEnvConditions(t *testing.T) {
	originalDetector := detectMatchContext
	t.Cleanup(func() { detectMatchContext = originalDetector })
	detectMatchContext = func(reqs matchRequirements) matchContext {
		return matchContext{
			LocalIPs:  []netip.Addr{netip.MustParseAddr("10.0.0.10")},
			Gateways:  []netip.Addr{netip.MustParseAddr("10.0.0.1")},
			OS:        "darwin",
			Terms:     []string{"xterm-256color"},
			EnvNames:  []string{"ROLE", "MODE"},
			EnvValues: map[string]string{"ROLE": "ops", "MODE": "debug"},
		}
	}

	cfg := Config{
		Server: map[string]ServerConfig{
			"app": {
				Addr:   "192.0.2.10",
				User:   "demo",
				Pass:   "secret",
				Ignore: true,
				Match: map[string]ServerMatchConfig{
					"negative": {
						When: ServerMatchWhen{
							LocalIPNotIn: []string{"192.168.0.0/16"},
							GatewayIn:    []string{"10.0.0.0/24"},
							OSIn:         []string{"darwin"},
							TermIn:       []string{"xterm-256color"},
							EnvIn:        []string{"ROLE"},
							EnvValueIn:   []string{"MODE=debug"},
						},
						Ignore:      false,
						Addr:        "203.0.113.10",
						definedKeys: map[string]bool{"ignore": true, "addr": true},
					},
				},
			},
		},
	}

	if err := cfg.ResolveConditionalMatches(); err != nil {
		t.Fatalf("ResolveConditionalMatches() error = %v", err)
	}
	if cfg.Server["app"].Ignore {
		t.Fatalf("Ignore = true, want false")
	}
	if cfg.Server["app"].Addr != "203.0.113.10" {
		t.Fatalf("Addr = %q", cfg.Server["app"].Addr)
	}
}

func TestWhenMatchesGatewayLookupFailureDoesNotMatchPositiveGatewayRule(t *testing.T) {
	ok := whenMatches(ServerMatchWhen{
		GatewayIn: []string{"10.0.0.0/24"},
	}, "app", "broken", matchContext{
		GatewayErr: assertErr("lookup failed"),
	})
	if ok {
		t.Fatal("whenMatches() = true, want false")
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
