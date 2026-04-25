package main

import (
	"testing"

	"github.com/blacknon/lssh/providerapi"
)

func TestTelnetDescribe(t *testing.T) {
	result, err := telnetDescribe(providerapi.ConnectorDescribeParams{
		Target: providerapi.ConnectorTarget{
			Config: map[string]interface{}{
				"addr": "router.local",
			},
		},
	})
	if err != nil {
		t.Fatalf("telnetDescribe() error = %v", err)
	}

	if !result.Capabilities["shell"].Supported {
		t.Fatal("shell capability = unsupported, want supported")
	}
	if result.Capabilities["exec"].Supported {
		t.Fatal("exec capability = supported, want unsupported")
	}
}

func TestTelnetPrepareShell(t *testing.T) {
	result, err := telnetPrepare(providerapi.ConnectorPrepareParams{
		Target: providerapi.ConnectorTarget{
			Config: map[string]interface{}{
				"addr": "router.local",
				"port": "2323",
				"user": "admin",
			},
		},
		Operation: providerapi.ConnectorOperation{Name: "shell"},
	})
	if err != nil {
		t.Fatalf("telnetPrepare() error = %v", err)
	}
	if !result.Supported {
		t.Fatal("Supported = false, want true")
	}
	if result.Plan.Kind != "provider-managed" {
		t.Fatalf("Plan.Kind = %q, want %q", result.Plan.Kind, "provider-managed")
	}
	if got := result.Plan.Details["runtime_dial_address"]; got != "router.local:2323" {
		t.Fatalf("Plan.Details[runtime_dial_address] = %v, want router.local:2323", got)
	}
}
