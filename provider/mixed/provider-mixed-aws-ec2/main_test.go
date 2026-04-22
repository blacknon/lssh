package main

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
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

func TestAWSDescribeInstanceInformationInputUsesAPIMinimumMaxResults(t *testing.T) {
	input := awsDescribeInstanceInformationInput("i-0123456789abcdef0")
	if input.MaxResults == nil {
		t.Fatal("MaxResults = nil, want non-nil")
	}
	if got := aws.ToInt32(input.MaxResults); got != awsSSMDescribeInstanceInformationMinResults {
		t.Fatalf("MaxResults = %d, want %d", got, awsSSMDescribeInstanceInformationMinResults)
	}
	if len(input.Filters) != 1 {
		t.Fatalf("len(Filters) = %d, want 1", len(input.Filters))
	}
	if got := aws.ToString(input.Filters[0].Key); got != "InstanceIds" {
		t.Fatalf("Filters[0].Key = %q, want %q", got, "InstanceIds")
	}
}

func TestAWSAddrStrategyDefaultsToPrivateFirst(t *testing.T) {
	if got := awsAddrStrategy(map[string]interface{}{}); got != "private_first" {
		t.Fatalf("awsAddrStrategy() = %q, want private_first", got)
	}
	if got := awsAddrStrategy(map[string]interface{}{"addr_strategy": "unknown"}); got != "private_first" {
		t.Fatalf("awsAddrStrategy(unknown) = %q, want private_first", got)
	}
}

func TestAWSSelectAddress(t *testing.T) {
	tests := []struct {
		name      string
		privateIP string
		publicIP  string
		strategy  string
		want      string
	}{
		{name: "private first", privateIP: "10.0.0.10", publicIP: "54.0.0.10", strategy: "private_first", want: "10.0.0.10"},
		{name: "public first", privateIP: "10.0.0.10", publicIP: "54.0.0.10", strategy: "public_first", want: "54.0.0.10"},
		{name: "private only", privateIP: "10.0.0.10", publicIP: "54.0.0.10", strategy: "private_only", want: "10.0.0.10"},
		{name: "public only", privateIP: "10.0.0.10", publicIP: "54.0.0.10", strategy: "public_only", want: "54.0.0.10"},
		{name: "public first fallback", privateIP: "10.0.0.10", publicIP: "", strategy: "public_first", want: "10.0.0.10"},
		{name: "private first fallback", privateIP: "", publicIP: "54.0.0.10", strategy: "private_first", want: "54.0.0.10"},
		{name: "public only empty", privateIP: "10.0.0.10", publicIP: "", strategy: "public_only", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := awsSelectAddress(tt.privateIP, tt.publicIP, tt.strategy); got != tt.want {
				t.Fatalf("awsSelectAddress() = %q, want %q", got, tt.want)
			}
		})
	}
}
