package main

import (
	"testing"

	"github.com/blacknon/lssh/providerapi"
)

func TestWinRMDescribe(t *testing.T) {
	result, err := winrmDescribe(providerapi.ConnectorDescribeParams{
		Config: map[string]interface{}{
			"enable_shell": "true",
		},
		Target: providerapi.ConnectorTarget{
			Config: map[string]interface{}{
				"addr": "windows.local",
				"user": "Administrator",
			},
		},
	})
	if err != nil {
		t.Fatalf("winrmDescribe() error = %v", err)
	}

	if !result.Capabilities["exec"].Supported {
		t.Fatal("exec capability = unsupported, want supported")
	}
	if !result.Capabilities["shell"].Supported {
		t.Fatal("shell capability = unsupported, want supported")
	}
	if result.Capabilities["port_forward_local"].Supported {
		t.Fatal("port_forward_local capability = supported, want unsupported")
	}
	if result.Capabilities["exec_pty"].Supported {
		t.Fatal("exec_pty capability = supported, want unsupported")
	}
}

func TestWinRMPrepareExec(t *testing.T) {
	result, err := winrmPrepare(providerapi.ConnectorPrepareParams{
		Target: providerapi.ConnectorTarget{
			Config: map[string]interface{}{
				"addr":      "windows.local",
				"user":      "Administrator",
				"pass":      "secret",
				"transport": "https",
			},
		},
		Operation: providerapi.ConnectorOperation{
			Name:    "exec",
			Command: []string{"hostname"},
		},
	})
	if err != nil {
		t.Fatalf("winrmPrepare() error = %v", err)
	}
	if !result.Supported {
		t.Fatal("Supported = false, want true")
	}
	if result.Plan.Kind != "provider-managed" {
		t.Fatalf("Plan.Kind = %q, want %q", result.Plan.Kind, "provider-managed")
	}
}
