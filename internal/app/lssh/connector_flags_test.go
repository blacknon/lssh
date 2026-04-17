package lssh

import (
	"strings"
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
)

func TestValidateConnectorShellOptionsRejectsMutuallyExclusiveFlags(t *testing.T) {
	err := validateConnectorShellOptions(connectorFlagOptions{
		AttachSession: "session-123",
		Detach:        true,
	}, []string{"aws:web-01"}, conf.Config{})
	if err == nil {
		t.Fatal("validateConnectorShellOptions() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "--attach and --detach") {
		t.Fatalf("validateConnectorShellOptions() error = %q, want attach/detach conflict", err)
	}
}

func TestValidateConnectorShellOptionsRejectsNonConnectorServer(t *testing.T) {
	err := validateConnectorShellOptions(connectorFlagOptions{
		AttachSession: "session-123",
	}, []string{"ssh-only"}, conf.Config{
		Server: map[string]conf.ServerConfig{
			"ssh-only": {ProviderName: "ssh-only-provider"},
		},
		Provider: map[string]map[string]interface{}{
			"ssh-only-provider": {
				"enabled":      true,
				"capabilities": []interface{}{"inventory"},
			},
		},
	})
	if err == nil {
		t.Fatal("validateConnectorShellOptions() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "connector-backed hosts") {
		t.Fatalf("validateConnectorShellOptions() error = %q, want connector-backed hosts", err)
	}
}

func TestValidateConnectorShellOptionsAllowsSingleConnectorShell(t *testing.T) {
	err := validateConnectorShellOptions(connectorFlagOptions{
		AttachSession: "session-123",
	}, []string{"aws:web-01"}, conf.Config{
		Server: map[string]conf.ServerConfig{
			"aws:web-01": {ProviderName: "aws"},
		},
		Provider: map[string]map[string]interface{}{
			"aws": {
				"enabled":      true,
				"capabilities": []interface{}{"inventory", "connector"},
			},
		},
	})
	if err != nil {
		t.Fatalf("validateConnectorShellOptions() error = %v, want nil", err)
	}
}
