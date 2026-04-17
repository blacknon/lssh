package main

import (
	"context"
	"testing"

	"github.com/blacknon/lssh/internal/providerapi"
)

func TestAWSConnectorDescribeSupported(t *testing.T) {
	originalProbe := probeSSMTarget
	defer func() { probeSSMTarget = originalProbe }()
	probeSSMTarget = func(ctx context.Context, raw map[string]interface{}, target awsSSMTargetConfig) (awsSSMProbeResult, error) {
		return awsSSMProbeResult{Managed: true, Online: true, Platform: "linux"}, nil
	}

	result, err := awsConnectorDescribe(providerapi.ConnectorDescribeParams{
		Config: map[string]interface{}{},
		Target: providerapi.ConnectorTarget{
			Name: "aws:web-01",
			Meta: map[string]string{
				"instance_id": "i-0123456789abcdef0",
				"region":      "ap-northeast-1",
				"platform":    "linux",
			},
		},
	})
	if err != nil {
		t.Fatalf("awsConnectorDescribe() error = %v", err)
	}
	if !result.Capabilities["shell"].Supported {
		t.Fatal("shell capability = unsupported, want supported")
	}
	if !result.Capabilities["exec"].Supported {
		t.Fatal("exec capability = unsupported, want supported")
	}
	if !result.Capabilities["exec_pty"].Supported {
		t.Fatal("exec_pty capability = unsupported, want supported")
	}
}

func TestAWSConnectorDescribeMissingMetadata(t *testing.T) {
	result, err := awsConnectorDescribe(providerapi.ConnectorDescribeParams{
		Target: providerapi.ConnectorTarget{Name: "aws:web-01"},
	})
	if err != nil {
		t.Fatalf("awsConnectorDescribe() error = %v", err)
	}
	if result.Capabilities["shell"].Supported {
		t.Fatal("shell capability = supported, want unsupported")
	}
}

func TestAWSConnectorPrepareExec(t *testing.T) {
	originalProbe := probeSSMTarget
	defer func() { probeSSMTarget = originalProbe }()
	probeSSMTarget = func(ctx context.Context, raw map[string]interface{}, target awsSSMTargetConfig) (awsSSMProbeResult, error) {
		return awsSSMProbeResult{Managed: true, Online: true, Platform: "windows"}, nil
	}

	result, err := awsConnectorPrepare(providerapi.ConnectorPrepareParams{
		Target: providerapi.ConnectorTarget{
			Name: "aws:web-01",
			Meta: map[string]string{
				"instance_id": "i-0123456789abcdef0",
				"region":      "ap-northeast-1",
				"platform":    "windows",
			},
		},
		Operation: providerapi.ConnectorOperation{
			Name:    "exec",
			Command: []string{"hostname"},
		},
	})
	if err != nil {
		t.Fatalf("awsConnectorPrepare() error = %v", err)
	}
	if !result.Supported {
		t.Fatal("Supported = false, want true")
	}
	if result.Plan.Kind != "provider-managed" {
		t.Fatalf("Plan.Kind = %q, want %q", result.Plan.Kind, "provider-managed")
	}
	if got := result.Plan.Details["connector"]; got != "aws-ssm" {
		t.Fatalf("Plan.Details[connector] = %v, want aws-ssm", got)
	}
}

func TestAWSConnectorPrepareShellAttach(t *testing.T) {
	originalProbe := probeSSMTarget
	defer func() { probeSSMTarget = originalProbe }()
	probeSSMTarget = func(ctx context.Context, raw map[string]interface{}, target awsSSMTargetConfig) (awsSSMProbeResult, error) {
		return awsSSMProbeResult{Managed: true, Online: true, Platform: "linux"}, nil
	}

	result, err := awsConnectorPrepare(providerapi.ConnectorPrepareParams{
		Target: providerapi.ConnectorTarget{
			Name: "aws:web-01",
			Meta: map[string]string{
				"instance_id": "i-0123456789abcdef0",
				"region":      "ap-northeast-1",
				"platform":    "linux",
			},
		},
		Operation: providerapi.ConnectorOperation{
			Name: "shell",
			Options: map[string]interface{}{
				"attach":     true,
				"session_id": "session-123",
			},
		},
	})
	if err != nil {
		t.Fatalf("awsConnectorPrepare() error = %v", err)
	}
	if got := result.Plan.Details["session_action"]; got != "attach" {
		t.Fatalf("Plan.Details[session_action] = %v, want attach", got)
	}
	if got := result.Plan.Details["session_id"]; got != "session-123" {
		t.Fatalf("Plan.Details[session_id] = %v, want session-123", got)
	}
}

func TestAWSConnectorPrepareRejectsAttachDetachConflict(t *testing.T) {
	originalProbe := probeSSMTarget
	defer func() { probeSSMTarget = originalProbe }()
	probeSSMTarget = func(ctx context.Context, raw map[string]interface{}, target awsSSMTargetConfig) (awsSSMProbeResult, error) {
		return awsSSMProbeResult{Managed: true, Online: true, Platform: "linux"}, nil
	}

	_, err := awsConnectorPrepare(providerapi.ConnectorPrepareParams{
		Target: providerapi.ConnectorTarget{
			Name: "aws:web-01",
			Meta: map[string]string{
				"instance_id": "i-0123456789abcdef0",
				"region":      "ap-northeast-1",
				"platform":    "linux",
			},
		},
		Operation: providerapi.ConnectorOperation{
			Name: "shell",
			Options: map[string]interface{}{
				"attach": true,
				"detach": true,
			},
		},
	})
	if err == nil {
		t.Fatal("awsConnectorPrepare() error = nil, want non-nil")
	}
}
