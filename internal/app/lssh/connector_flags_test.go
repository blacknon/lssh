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
	}, []string{"connector-host"}, conf.Config{})
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
			"ssh-only": {},
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
	}, []string{"connector-host"}, conf.Config{
		Server: map[string]conf.ServerConfig{
			"connector-host": {ConnectorName: "session-capable"},
		},
	})
	if err != nil {
		t.Fatalf("validateConnectorShellOptions() error = %v, want nil", err)
	}
}

func TestValidateConnectorShellOptionsDefersConnectorSpecificChecks(t *testing.T) {
	err := validateConnectorShellOptions(connectorFlagOptions{
		AttachSession: "session-123",
	}, []string{"web01"}, conf.Config{
		Server: map[string]conf.ServerConfig{
			"web01": {ConnectorName: "openssh"},
		},
	})
	if err != nil {
		t.Fatalf("validateConnectorShellOptions() error = %v, want nil", err)
	}
}
