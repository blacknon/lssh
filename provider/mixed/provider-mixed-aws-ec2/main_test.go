package main

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/blacknon/lssh/providerapi"
)

func TestAWSHealthCheckScopes(t *testing.T) {
	tests := []struct {
		name         string
		config       map[string]interface{}
		wantCheckEC2 bool
		wantCheckSSM bool
		wantMessage  string
	}{
		{
			name:         "default mixed provider behavior",
			config:       map[string]interface{}{},
			wantCheckEC2: true,
			wantCheckSSM: true,
			wantMessage:  "aws mixed provider can access EC2 and SSM",
		},
		{
			name: "inventory only capability",
			config: map[string]interface{}{
				"capabilities": []interface{}{"inventory"},
			},
			wantCheckEC2: true,
			wantCheckSSM: false,
			wantMessage:  "aws mixed provider can access EC2",
		},
		{
			name: "inventory and connector capabilities",
			config: map[string]interface{}{
				"capabilities": []interface{}{"inventory", "connector"},
			},
			wantCheckEC2: true,
			wantCheckSSM: true,
			wantMessage:  "aws mixed provider can access EC2 and SSM",
		},
		{
			name: "aws eice only connector",
			config: map[string]interface{}{
				"connector_names": []interface{}{"aws-eice"},
			},
			wantCheckEC2: true,
			wantCheckSSM: false,
			wantMessage:  "aws mixed provider can access EC2",
		},
		{
			name: "aws ssm connector",
			config: map[string]interface{}{
				"connector_names": []interface{}{"aws-ssm"},
			},
			wantCheckEC2: true,
			wantCheckSSM: true,
			wantMessage:  "aws mixed provider can access EC2 and SSM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEC2, gotSSM := awsHealthCheckScopes(tt.config)
			if gotEC2 != tt.wantCheckEC2 || gotSSM != tt.wantCheckSSM {
				t.Fatalf("awsHealthCheckScopes() = (%t, %t), want (%t, %t)", gotEC2, gotSSM, tt.wantCheckEC2, tt.wantCheckSSM)
			}
			if got := awsHealthCheckMessage(gotEC2, gotSSM); got != tt.wantMessage {
				t.Fatalf("awsHealthCheckMessage() = %q, want %q", got, tt.wantMessage)
			}
		})
	}
}

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

func TestAWSConnectorPrepareShellNativeRuntime(t *testing.T) {
	originalProbe := probeSSMTarget
	defer func() { probeSSMTarget = originalProbe }()
	probeSSMTarget = func(ctx context.Context, raw map[string]interface{}, target awsSSMTargetConfig) (awsSSMProbeResult, error) {
		return awsSSMProbeResult{Managed: true, Online: true, Platform: "linux"}, nil
	}

	result, err := awsConnectorPrepare(providerapi.ConnectorPrepareParams{
		Config: map[string]interface{}{
			"ssm_shell_runtime": "native",
		},
		Target: providerapi.ConnectorTarget{
			Name: "aws:web-01",
			Meta: map[string]string{
				"instance_id": "i-0123456789abcdef0",
				"region":      "ap-northeast-1",
				"platform":    "linux",
			},
		},
		Operation: providerapi.ConnectorOperation{Name: "shell"},
	})
	if err != nil {
		t.Fatalf("awsConnectorPrepare() error = %v", err)
	}
	if got := result.Plan.Details["shell_runtime"]; got != "native" {
		t.Fatalf("Plan.Details[shell_runtime] = %v, want native", got)
	}
}

func TestAWSConnectorPreparePortForwardFallsBackToShellNativeRuntime(t *testing.T) {
	originalProbe := probeSSMTarget
	defer func() { probeSSMTarget = originalProbe }()
	probeSSMTarget = func(ctx context.Context, raw map[string]interface{}, target awsSSMTargetConfig) (awsSSMProbeResult, error) {
		return awsSSMProbeResult{Managed: true, Online: true, Platform: "linux"}, nil
	}

	result, err := awsConnectorPrepare(providerapi.ConnectorPrepareParams{
		Config: map[string]interface{}{
			"ssm_shell_runtime": "native",
		},
		Target: providerapi.ConnectorTarget{
			Name: "aws:web-01",
			Meta: map[string]string{
				"instance_id": "i-0123456789abcdef0",
				"region":      "ap-northeast-1",
				"platform":    "linux",
			},
		},
		Operation: providerapi.ConnectorOperation{
			Name: "tcp_dial_transport",
			Options: map[string]interface{}{
				"target_host": "api.internal",
				"target_port": "443",
			},
		},
	})
	if err != nil {
		t.Fatalf("awsConnectorPrepare() error = %v", err)
	}
	if got := result.Plan.Details["port_forward_runtime"]; got != "native" {
		t.Fatalf("Plan.Details[port_forward_runtime] = %v, want native", got)
	}
}

func TestAWSConnectorDescribePortForwardLocalPluginRuntime(t *testing.T) {
	originalProbe := probeSSMTarget
	defer func() { probeSSMTarget = originalProbe }()
	probeSSMTarget = func(ctx context.Context, raw map[string]interface{}, target awsSSMTargetConfig) (awsSSMProbeResult, error) {
		return awsSSMProbeResult{Managed: true, Online: true, Platform: "linux"}, nil
	}

	result, err := awsConnectorDescribe(providerapi.ConnectorDescribeParams{
		Config: map[string]interface{}{
			"ssm_port_forward_runtime": "plugin",
		},
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
	if !result.Capabilities["port_forward_local"].Supported {
		t.Fatal("port_forward_local capability = unsupported, want supported")
	}
	if !result.Capabilities["tcp_dial_transport"].Supported {
		t.Fatal("tcp_dial_transport capability = unsupported, want supported")
	}
}

func TestAWSConnectorPreparePortForwardLocal(t *testing.T) {
	originalProbe := probeSSMTarget
	defer func() { probeSSMTarget = originalProbe }()
	probeSSMTarget = func(ctx context.Context, raw map[string]interface{}, target awsSSMTargetConfig) (awsSSMProbeResult, error) {
		return awsSSMProbeResult{Managed: true, Online: true, Platform: "linux"}, nil
	}

	result, err := awsConnectorPrepare(providerapi.ConnectorPrepareParams{
		Config: map[string]interface{}{
			"ssm_port_forward_runtime": "plugin",
		},
		Target: providerapi.ConnectorTarget{
			Name: "aws:web-01",
			Meta: map[string]string{
				"instance_id": "i-0123456789abcdef0",
				"region":      "ap-northeast-1",
				"platform":    "linux",
			},
		},
		Operation: providerapi.ConnectorOperation{
			Name: "port_forward_local",
			Options: map[string]interface{}{
				"listen_host": "localhost",
				"listen_port": "15432",
				"target_host": "db.internal",
				"target_port": "5432",
			},
		},
	})
	if err != nil {
		t.Fatalf("awsConnectorPrepare() error = %v", err)
	}
	if !result.Supported {
		t.Fatal("Supported = false, want true")
	}
	if got := result.Plan.Details["session_mode"]; got != "port-forward-local" {
		t.Fatalf("Plan.Details[session_mode] = %v, want port-forward-local", got)
	}
	if got := result.Plan.Details["target_host"]; got != "db.internal" {
		t.Fatalf("Plan.Details[target_host] = %v, want db.internal", got)
	}
}

func TestAWSConnectorPreparePortForwardLocalNativeSupported(t *testing.T) {
	originalProbe := probeSSMTarget
	defer func() { probeSSMTarget = originalProbe }()
	probeSSMTarget = func(ctx context.Context, raw map[string]interface{}, target awsSSMTargetConfig) (awsSSMProbeResult, error) {
		return awsSSMProbeResult{Managed: true, Online: true, Platform: "linux"}, nil
	}

	result, err := awsConnectorPrepare(providerapi.ConnectorPrepareParams{
		Config: map[string]interface{}{
			"ssm_port_forward_runtime": "native",
		},
		Target: providerapi.ConnectorTarget{
			Name: "aws:web-01",
			Meta: map[string]string{
				"instance_id": "i-0123456789abcdef0",
				"region":      "ap-northeast-1",
				"platform":    "linux",
			},
		},
		Operation: providerapi.ConnectorOperation{
			Name: "port_forward_local",
			Options: map[string]interface{}{
				"listen_host": "localhost",
				"listen_port": "15432",
				"target_host": "db.internal",
				"target_port": "5432",
			},
		},
	})
	if err != nil {
		t.Fatalf("awsConnectorPrepare() error = %v", err)
	}
	if !result.Supported {
		t.Fatal("Supported = false, want true")
	}
	if got := result.Plan.Details["port_forward_runtime"]; got != "native" {
		t.Fatalf("Plan.Details[port_forward_runtime] = %v, want native", got)
	}
}

func TestAWSConnectorPrepareTCPDialTransport(t *testing.T) {
	originalProbe := probeSSMTarget
	defer func() { probeSSMTarget = originalProbe }()
	probeSSMTarget = func(ctx context.Context, raw map[string]interface{}, target awsSSMTargetConfig) (awsSSMProbeResult, error) {
		return awsSSMProbeResult{Managed: true, Online: true, Platform: "linux"}, nil
	}

	result, err := awsConnectorPrepare(providerapi.ConnectorPrepareParams{
		Config: map[string]interface{}{
			"ssm_port_forward_runtime": "plugin",
		},
		Target: providerapi.ConnectorTarget{
			Name: "aws:web-01",
			Meta: map[string]string{
				"instance_id": "i-0123456789abcdef0",
				"region":      "ap-northeast-1",
				"platform":    "linux",
			},
		},
		Operation: providerapi.ConnectorOperation{
			Name: "tcp_dial_transport",
			Options: map[string]interface{}{
				"target_host": "api.internal",
				"target_port": "443",
			},
		},
	})
	if err != nil {
		t.Fatalf("awsConnectorPrepare() error = %v", err)
	}
	if !result.Supported {
		t.Fatal("Supported = false, want true")
	}
	if got := result.Plan.Details["session_mode"]; got != "tcp-dial-transport" {
		t.Fatalf("Plan.Details[session_mode] = %v, want tcp-dial-transport", got)
	}
	if got := result.Plan.Details["target_host"]; got != "api.internal" {
		t.Fatalf("Plan.Details[target_host] = %v, want api.internal", got)
	}
}

func TestAWSConnectorPrepareTCPDialTransportNativeSupported(t *testing.T) {
	originalProbe := probeSSMTarget
	defer func() { probeSSMTarget = originalProbe }()
	probeSSMTarget = func(ctx context.Context, raw map[string]interface{}, target awsSSMTargetConfig) (awsSSMProbeResult, error) {
		return awsSSMProbeResult{Managed: true, Online: true, Platform: "linux"}, nil
	}

	result, err := awsConnectorPrepare(providerapi.ConnectorPrepareParams{
		Config: map[string]interface{}{
			"ssm_port_forward_runtime": "native",
		},
		Target: providerapi.ConnectorTarget{
			Name: "aws:web-01",
			Meta: map[string]string{
				"instance_id": "i-0123456789abcdef0",
				"region":      "ap-northeast-1",
				"platform":    "linux",
			},
		},
		Operation: providerapi.ConnectorOperation{
			Name: "tcp_dial_transport",
			Options: map[string]interface{}{
				"target_host": "api.internal",
				"target_port": "443",
			},
		},
	})
	if err != nil {
		t.Fatalf("awsConnectorPrepare() error = %v", err)
	}
	if !result.Supported {
		t.Fatal("Supported = false, want true")
	}
	if got := result.Plan.Details["port_forward_runtime"]; got != "native" {
		t.Fatalf("Plan.Details[port_forward_runtime] = %v, want native", got)
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

func TestAWSEICERuntimePrefersTargetConfig(t *testing.T) {
	got := awsEICERuntime(
		map[string]interface{}{"eice_runtime": "sdk"},
		map[string]interface{}{"eice_runtime": "command"},
	)
	if got != "command" {
		t.Fatalf("awsEICERuntime() = %q, want command", got)
	}
}

func TestAWSEICETargetFromParamsPrefersTargetConfig(t *testing.T) {
	target, missing := awsEICETargetFromParams(
		map[string]interface{}{
			"profile":                            "provider-profile",
			"instance_connect_endpoint_id":       "provider-eice",
			"instance_connect_endpoint_dns_name": "provider.example",
		},
		providerapi.ConnectorTarget{
			Config: map[string]interface{}{
				"profile":                            "target-profile",
				"instance_connect_endpoint_id":       "target-eice",
				"instance_connect_endpoint_dns_name": "target.example",
			},
			Meta: map[string]string{
				"instance_id": "i-0123456789abcdef0",
				"region":      "ap-northeast-1",
			},
		},
	)
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want empty", missing)
	}
	if target.Profile != "target-profile" {
		t.Fatalf("Profile = %q, want target-profile", target.Profile)
	}
	if target.EndpointID != "target-eice" {
		t.Fatalf("EndpointID = %q, want target-eice", target.EndpointID)
	}
	if target.EndpointDNSName != "target.example" {
		t.Fatalf("EndpointDNSName = %q, want target.example", target.EndpointDNSName)
	}
}
