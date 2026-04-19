package main

import (
	"runtime"
	"testing"

	"github.com/blacknon/lssh/internal/providerapi"
)

func TestOpenSSHDescribe(t *testing.T) {
	result, err := opensshDescribe(providerapi.ConnectorDescribeParams{
		Target: providerapi.ConnectorTarget{
			Config: map[string]interface{}{
				"addr": "example.internal",
				"user": "demo",
				"port": "2222",
			},
		},
	})
	if err != nil {
		t.Fatalf("opensshDescribe() error = %v", err)
	}

	if !result.Capabilities["shell"].Supported {
		t.Fatal("shell capability = unsupported, want supported")
	}
	if !result.Capabilities["sftp_transport"].Supported {
		t.Fatal("sftp_transport capability = unsupported, want supported")
	}
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		if !result.Capabilities["mount"].Supported {
			t.Fatal("mount capability = unsupported on supported unix, want supported")
		}
	} else {
		if result.Capabilities["mount"].Supported {
			t.Fatal("mount capability = supported, want unsupported")
		}
	}
}

func TestOpenSSHPrepareShell(t *testing.T) {
	result, err := opensshPrepare(providerapi.ConnectorPrepareParams{
		Config: map[string]interface{}{
			"ssh_path":                 "/usr/bin/ssh",
			"strict_host_key_checking": "accept-new",
		},
		Target: providerapi.ConnectorTarget{
			Config: map[string]interface{}{
				"addr": "example.internal",
				"user": "demo",
				"port": "2222",
				"key":  "/tmp/id_ed25519",
			},
		},
		Operation: providerapi.ConnectorOperation{Name: "shell"},
	})
	if err != nil {
		t.Fatalf("opensshPrepare() error = %v", err)
	}
	if !result.Supported {
		t.Fatal("Supported = false, want true")
	}
	if result.Plan.Kind != "command" {
		t.Fatalf("Plan.Kind = %q, want %q", result.Plan.Kind, "command")
	}
	if result.Plan.Program != "/usr/bin/ssh" {
		t.Fatalf("Plan.Program = %q, want %q", result.Plan.Program, "/usr/bin/ssh")
	}
	if len(result.Plan.Args) == 0 {
		t.Fatal("Plan.Args is empty")
	}
}

func TestOpenSSHPrepareSFTPTransport(t *testing.T) {
	result, err := opensshPrepare(providerapi.ConnectorPrepareParams{
		Target: providerapi.ConnectorTarget{
			Config: map[string]interface{}{
				"addr": "example.internal",
				"user": "demo",
			},
		},
		Operation: providerapi.ConnectorOperation{Name: "sftp_transport"},
	})
	if err != nil {
		t.Fatalf("opensshPrepare() error = %v", err)
	}
	if !result.Supported {
		t.Fatal("Supported = false, want true")
	}
	if result.Plan.Details["transport"] != "sftp_transport" {
		t.Fatalf("transport = %#v, want %q", result.Plan.Details["transport"], "sftp_transport")
	}
}

func TestOpenSSHPrepareMount(t *testing.T) {
	result, err := opensshPrepare(providerapi.ConnectorPrepareParams{
		Target: providerapi.ConnectorTarget{
			Config: map[string]interface{}{
				"addr": "example.internal",
				"user": "demo",
			},
		},
		Operation: providerapi.ConnectorOperation{Name: "mount"},
	})
	if err != nil {
		t.Fatalf("opensshPrepare() error = %v", err)
	}

	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		if !result.Supported {
			t.Fatal("Supported = false on supported unix, want true")
		}
		if result.Plan.Details["transport"] != "sftp_transport" {
			t.Fatalf("transport = %#v, want %q", result.Plan.Details["transport"], "sftp_transport")
		}
		return
	}

	if result.Supported {
		t.Fatal("Supported = true, want false outside linux")
	}
	if result.Plan.Details["reason"] == "" {
		t.Fatal("Plan.Details[reason] is empty")
	}
}
