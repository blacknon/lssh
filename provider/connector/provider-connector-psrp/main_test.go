package main

import (
	"os"
	"testing"

	"github.com/blacknon/lssh/providerapi"
)

func TestPSRPDescribe(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	result, err := psrpDescribe(providerapi.ConnectorDescribeParams{
		Config: map[string]interface{}{
			"enable_shell": "true",
			"pwsh_path":    exe,
		},
		Target: providerapi.ConnectorTarget{
			Config: map[string]interface{}{
				"addr": "windows.local",
				"user": "Administrator",
			},
		},
	})
	if err != nil {
		t.Fatalf("psrpDescribe() error = %v", err)
	}

	if !result.Capabilities["exec"].Supported {
		t.Fatal("exec capability = unsupported, want supported")
	}
	if !result.Capabilities["shell"].Supported {
		t.Fatal("shell capability = unsupported, want supported")
	}
	if result.Capabilities["exec_pty"].Supported {
		t.Fatal("exec_pty capability = supported, want unsupported")
	}
}

func TestPSRPPrepareExec(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	result, err := psrpPrepare(providerapi.ConnectorPrepareParams{
		Config: map[string]interface{}{
			"pwsh_path": exe,
		},
		Target: providerapi.ConnectorTarget{
			Config: map[string]interface{}{
				"addr": "windows.local",
				"user": "Administrator",
				"pass": "secret",
			},
		},
		Operation: providerapi.ConnectorOperation{
			Name:    "exec",
			Command: []string{"Get-Process"},
		},
	})
	if err != nil {
		t.Fatalf("psrpPrepare() error = %v", err)
	}
	if !result.Supported {
		t.Fatal("Supported = false, want true")
	}
	if result.Plan.Kind != "command" {
		t.Fatalf("Plan.Kind = %q, want %q", result.Plan.Kind, "command")
	}
	if got := result.Plan.Details["connector"]; got != "psrp" {
		t.Fatalf("Plan.Details[connector] = %v, want psrp", got)
	}
}

func TestPSRPDescribeLibraryRuntimeAdvertised(t *testing.T) {
	result, err := psrpDescribe(providerapi.ConnectorDescribeParams{
		Config: map[string]interface{}{
			"enable_shell": "true",
			"runtime":      "library",
		},
		Target: providerapi.ConnectorTarget{
			Config: map[string]interface{}{
				"addr": "windows.local",
				"user": "Administrator",
			},
		},
	})
	if err != nil {
		t.Fatalf("psrpDescribe() error = %v", err)
	}

	if !result.Capabilities["shell"].Supported {
		t.Fatal("shell capability = unsupported, want supported")
	}
}

func TestPSRPPrepareLibraryRuntimeReturnsHelperPlan(t *testing.T) {
	result, err := psrpPrepare(providerapi.ConnectorPrepareParams{
		Config: map[string]interface{}{
			"enable_shell": "true",
			"runtime":      "library",
		},
		Target: providerapi.ConnectorTarget{
			Config: map[string]interface{}{
				"addr": "windows.local",
				"user": "Administrator",
				"pass": "secret",
			},
		},
		Operation: providerapi.ConnectorOperation{
			Name: "shell",
		},
	})
	if err != nil {
		t.Fatalf("psrpPrepare() error = %v", err)
	}
	if !result.Supported {
		t.Fatal("Supported = false, want true")
	}
	if result.Plan.Kind != "command" {
		t.Fatalf("Plan.Kind = %q, want command", result.Plan.Kind)
	}
	if got := result.Plan.Details["runtime"]; got != "library" {
		t.Fatalf("Plan.Details[runtime] = %v, want library", got)
	}
	if len(result.Plan.Args) < 2 || result.Plan.Args[0] != "__psrp_helper" || result.Plan.Args[1] != "shell" {
		t.Fatalf("Plan.Args = %v, want helper shell invocation", result.Plan.Args)
	}
}
