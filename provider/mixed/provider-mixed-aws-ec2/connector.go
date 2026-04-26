package main

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/blacknon/lssh/providerapi"
	"github.com/blacknon/lssh/provider/connector/provider-connector-openssh/opensshlib"
	"github.com/blacknon/lssh/provider/mixed/provider-mixed-aws-ec2/eiceconnector"
)

type awsSSMTargetConfig struct {
	InstanceID                 string
	Region                     string
	Platform                   string
	ShellDocumentName          string
	InteractiveCommandDocument string
	RequireOnline              bool
}

type awsSSMProbeResult struct {
	Managed  bool
	Online   bool
	Platform string
}

var probeSSMTarget = defaultProbeSSMTarget

const awsSSMDescribeInstanceInformationMinResults int32 = 5

func awsConnectorDescribe(params providerapi.ConnectorDescribeParams) (providerapi.ConnectorDescribeResult, error) {
	if awsConnectorName(params) == "aws-eice" {
		return awsEICEConnectorDescribe(params)
	}
	target, missing := awsSSMTargetFromParams(params.Config, params.Target)
	if len(missing) > 0 {
		return awsUnsupportedSSMCapabilities(
			fmt.Sprintf("aws ssm target metadata is missing: %s", strings.Join(missing, ", ")),
		), nil
	}

	probe, err := probeSSMTarget(context.Background(), params.Config, target)
	if err != nil {
		return providerapi.ConnectorDescribeResult{}, err
	}

	return providerapi.ConnectorDescribeResult{
		Capabilities: map[string]providerapi.ConnectorCapability{
			"shell": {
				Supported: probe.Managed && probe.Online,
				Reason:    awsSSMUnsupportedReason(probe, "interactive session requires an SSM-managed online instance"),
				Requires:  []string{"aws:ssm_managed_instance"},
				Preferred: true,
			},
			"exec": {
				Supported: probe.Managed && probe.Online,
				Reason:    awsSSMUnsupportedReason(probe, "command execution requires an SSM-managed online instance"),
				Requires:  []string{"aws:ssm_managed_instance"},
				Preferred: true,
			},
			"exec_pty": {
				Supported: probe.Managed && probe.Online,
				Reason:    awsSSMUnsupportedReason(probe, "pty-like command execution requires an SSM-managed online instance"),
				Requires:  []string{"aws:ssm_managed_instance"},
			},
			"upload": {
				Supported: false,
				Reason:    "aws ssm native file transfer is not implemented in the first connector wave",
			},
			"download": {
				Supported: false,
				Reason:    "aws ssm native file transfer is not implemented in the first connector wave",
			},
			"mount": {
				Supported: false,
				Reason:    "aws ssm does not provide mount semantics",
			},
			"port_forward_local": {
				Supported: probe.Managed && probe.Online,
				Reason: awsSSMLocalPortForwardReason(
					probe,
					awsSSMPortForwardRuntime(params.Config),
				),
				Requires: []string{"aws:ssm_managed_instance"},
			},
			"tcp_dial_transport": {
				Supported: probe.Managed && probe.Online,
				Reason: awsSSMTCPDialTransportReason(
					probe,
					awsSSMPortForwardRuntime(params.Config),
				),
				Requires: []string{"aws:ssm_managed_instance"},
			},
			"port_forward_remote": {
				Supported: false,
				Reason:    "aws ssm remote port forwarding is not implemented in the first connector wave",
			},
			"agent_forward": {
				Supported: false,
				Reason:    "aws ssm does not provide ssh agent forwarding",
			},
		},
	}, nil
}

func awsConnectorPrepare(params providerapi.ConnectorPrepareParams) (providerapi.ConnectorPrepareResult, error) {
	if awsConnectorNameFromPrepare(params) == "aws-eice" {
		return awsEICEConnectorPrepare(params)
	}
	target, missing := awsSSMTargetFromParams(params.Config, params.Target)
	if len(missing) > 0 {
		return providerapi.ConnectorPrepareResult{
			Supported: false,
			Plan: providerapi.ConnectorPlan{
				Kind: "provider-managed",
				Details: map[string]interface{}{
					"connector": "aws-ssm",
					"reason":    fmt.Sprintf("aws ssm target metadata is missing: %s", strings.Join(missing, ", ")),
				},
			},
		}, nil
	}

	probe, err := probeSSMTarget(context.Background(), params.Config, target)
	if err != nil {
		return providerapi.ConnectorPrepareResult{}, err
	}

	sessionAction, sessionID, err := awsSSMSessionActionFromOperation(params.Operation)
	if err != nil {
		return providerapi.ConnectorPrepareResult{}, err
	}

	supported := probe.Managed && probe.Online
	plan := providerapi.ConnectorPlan{
		Kind: "provider-managed",
		Details: map[string]interface{}{
			"connector":                "aws-ssm",
			"instance_id":              target.InstanceID,
			"region":                   target.Region,
			"platform":                 awsFirstNonEmpty(probe.Platform, target.Platform),
			"profile":                  providerapi.String(params.Config, "profile"),
			"shared_config_files":      providerapi.StringSlice(params.Config, "shared_config_files"),
			"shared_credentials_files": providerapi.StringSlice(params.Config, "shared_credentials_files"),
			"poll_interval_sec":        2,
			"timeout_sec":              60,
			"operation":                params.Operation.Name,
			"shell_runtime":            awsSSMShellRuntime(params.Config),
			"port_forward_runtime":     awsSSMPortForwardRuntime(params.Config),
		},
	}

	switch params.Operation.Name {
	case "shell":
		plan.Details["session_mode"] = "shell"
		plan.Details["session_action"] = sessionAction
		if sessionID != "" {
			plan.Details["session_id"] = sessionID
		}
		if target.ShellDocumentName != "" {
			plan.Details["document_name"] = target.ShellDocumentName
		}
	case "exec":
		plan.Details["session_mode"] = "send-command"
		plan.Details["command"] = params.Operation.Command
		plan.Details["command_line"] = awsFirstNonEmpty(
			awsOptionString(params.Operation.Options, "command_line"),
			strings.TrimSpace(strings.Join(params.Operation.Command, " ")),
		)
		if strings.EqualFold(awsFirstNonEmpty(probe.Platform, target.Platform), "windows") {
			plan.Details["command_document_name"] = "AWS-RunPowerShellScript"
		} else {
			plan.Details["command_document_name"] = "AWS-RunShellScript"
		}
	case "exec_pty":
		plan.Details["session_mode"] = "interactive-command"
		plan.Details["command"] = params.Operation.Command
		if target.InteractiveCommandDocument != "" {
			plan.Details["document_name"] = target.InteractiveCommandDocument
		}
	case "port_forward_local":
		spec, specErr := awsLocalPortForwardSpecFromOperation(params.Operation)
		if specErr != nil {
			return providerapi.ConnectorPrepareResult{}, specErr
		}
		plan.Details["session_mode"] = "port-forward-local"
		plan.Details["listen_host"] = spec.ListenHost
		plan.Details["listen_port"] = spec.ListenPort
		plan.Details["target_host"] = spec.TargetHost
		plan.Details["target_port"] = spec.TargetPort
		if documentName := providerapi.String(params.Config, "ssm_port_forward_document"); documentName != "" {
			plan.Details["document_name"] = documentName
		}
	case "tcp_dial_transport":
		spec, specErr := awsDialTransportSpecFromOperation(params.Operation)
		if specErr != nil {
			return providerapi.ConnectorPrepareResult{}, specErr
		}
		plan.Details["session_mode"] = "tcp-dial-transport"
		plan.Details["target_host"] = spec.TargetHost
		plan.Details["target_port"] = spec.TargetPort
		if documentName := providerapi.String(params.Config, "ssm_port_forward_document"); documentName != "" {
			plan.Details["document_name"] = documentName
		}
	default:
		return providerapi.ConnectorPrepareResult{
			Supported: false,
			Plan: providerapi.ConnectorPlan{
				Kind: "provider-managed",
				Details: map[string]interface{}{
					"connector": "aws-ssm",
					"reason":    fmt.Sprintf("operation %q is not supported by the aws ssm connector", params.Operation.Name),
				},
			},
		}, nil
	}

	if !supported {
		if _, ok := plan.Details["reason"]; !ok {
			plan.Details["reason"] = awsSSMUnsupportedReason(probe, "aws ssm target is not online")
		}
	}

	return providerapi.ConnectorPrepareResult{
		Supported: supported,
		Plan:      plan,
	}, nil
}

func awsSSMSessionActionFromOperation(operation providerapi.ConnectorOperation) (string, string, error) {
	attach := awsOptionBool(operation.Options, "attach")
	detach := awsOptionBool(operation.Options, "detach")
	sessionID := strings.TrimSpace(awsOptionString(operation.Options, "session_id"))

	if operation.Name != "shell" && (attach || detach || sessionID != "") {
		return "", "", fmt.Errorf("attach/detach options are only supported for shell operations")
	}
	if attach && detach {
		return "", "", fmt.Errorf("attach and detach cannot be used together")
	}
	if (attach || detach) && len(operation.Command) > 0 {
		return "", "", fmt.Errorf("attach/detach options cannot be used with command arguments")
	}
	if attach {
		if sessionID == "" {
			return "", "", fmt.Errorf("attach requires session_id")
		}
		return "attach", sessionID, nil
	}
	if sessionID != "" {
		return "", "", fmt.Errorf("session_id can only be used together with attach")
	}
	if detach {
		return "detach", "", nil
	}

	return "start", "", nil
}

func awsSSMTargetFromParams(config map[string]interface{}, target providerapi.ConnectorTarget) (awsSSMTargetConfig, []string) {
	instanceID := awsFirstNonEmpty(
		target.Meta["instance_id"],
		providerapi.String(target.Config, "instance_id"),
		providerapi.String(config, "instance_id"),
	)
	region := awsFirstNonEmpty(
		target.Meta["region"],
		providerapi.String(target.Config, "region"),
		providerapi.String(config, "region"),
	)
	platform := strings.ToLower(awsFirstNonEmpty(
		target.Meta["platform"],
		providerapi.String(target.Config, "platform"),
		providerapi.String(config, "platform"),
	))
	result := awsSSMTargetConfig{
		InstanceID:                 instanceID,
		Region:                     region,
		Platform:                   platform,
		ShellDocumentName:          providerapi.String(config, "ssm_shell_document"),
		InteractiveCommandDocument: providerapi.String(config, "ssm_interactive_command_document"),
		RequireOnline:              awsBool(config, "ssm_require_online", true),
	}

	var missing []string
	if result.InstanceID == "" {
		missing = append(missing, "instance_id")
	}
	if result.Region == "" {
		missing = append(missing, "region")
	}
	return result, missing
}

func awsSSMShellRuntime(config map[string]interface{}) string {
	switch strings.ToLower(strings.TrimSpace(providerapi.String(config, "ssm_shell_runtime"))) {
	case "", "plugin":
		return "plugin"
	case "native":
		return "native"
	default:
		return "plugin"
	}
}

func awsSSMPortForwardRuntime(config map[string]interface{}) string {
	switch value := strings.ToLower(strings.TrimSpace(providerapi.String(config, "ssm_port_forward_runtime"))); value {
	case "":
		return awsSSMShellRuntime(config)
	case "plugin":
		return "plugin"
	case "native":
		return "native"
	default:
		return awsSSMShellRuntime(config)
	}
}

func awsSSMLocalPortForwardReason(probe awsSSMProbeResult, runtime string) string {
	return awsSSMUnsupportedReason(probe, "local port forwarding requires an SSM-managed online instance")
}

func awsSSMTCPDialTransportReason(probe awsSSMProbeResult, runtime string) string {
	return awsSSMUnsupportedReason(probe, "dial transport requires an SSM-managed online instance")
}

type awsLocalPortForwardSpec struct {
	ListenHost string
	ListenPort string
	TargetHost string
	TargetPort string
}

type awsDialTransportSpec struct {
	TargetHost string
	TargetPort string
}

func awsLocalPortForwardSpecFromOperation(operation providerapi.ConnectorOperation) (awsLocalPortForwardSpec, error) {
	listenHost := strings.TrimSpace(awsOptionString(operation.Options, "listen_host"))
	listenPort := strings.TrimSpace(awsOptionString(operation.Options, "listen_port"))
	targetHost := strings.TrimSpace(awsOptionString(operation.Options, "target_host"))
	targetPort := strings.TrimSpace(awsOptionString(operation.Options, "target_port"))

	if listenHost == "" {
		listenHost = "localhost"
	}
	if listenPort == "" || targetHost == "" || targetPort == "" {
		return awsLocalPortForwardSpec{}, fmt.Errorf("local port forward requires listen_port, target_host, and target_port")
	}
	if !awsIsLoopbackHost(listenHost) {
		return awsLocalPortForwardSpec{}, fmt.Errorf("aws ssm local port forward only supports localhost bind addresses")
	}
	return awsLocalPortForwardSpec{
		ListenHost: listenHost,
		ListenPort: listenPort,
		TargetHost: targetHost,
		TargetPort: targetPort,
	}, nil
}

func awsDialTransportSpecFromOperation(operation providerapi.ConnectorOperation) (awsDialTransportSpec, error) {
	targetHost := strings.TrimSpace(awsOptionString(operation.Options, "target_host"))
	targetPort := strings.TrimSpace(awsOptionString(operation.Options, "target_port"))
	if targetHost == "" || targetPort == "" {
		return awsDialTransportSpec{}, fmt.Errorf("tcp dial transport requires target_host and target_port")
	}
	return awsDialTransportSpec{
		TargetHost: targetHost,
		TargetPort: targetPort,
	}, nil
}

func awsIsLoopbackHost(host string) bool {
	normalized := strings.TrimSpace(host)
	if normalized == "" {
		return true
	}
	switch strings.ToLower(normalized) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	ip := net.ParseIP(normalized)
	return ip != nil && ip.IsLoopback()
}

func awsConnectorName(params providerapi.ConnectorDescribeParams) string {
	connectorName := strings.TrimSpace(providerapi.String(params.Target.Config, "connector_name"))
	if connectorName != "" {
		return connectorName
	}
	return strings.TrimSpace(providerapi.String(params.Config, "default_connector_name"))
}

func awsConnectorNameFromPrepare(params providerapi.ConnectorPrepareParams) string {
	connectorName := strings.TrimSpace(providerapi.String(params.Target.Config, "connector_name"))
	if connectorName != "" {
		return connectorName
	}
	return strings.TrimSpace(providerapi.String(params.Config, "default_connector_name"))
}

type awsEICETargetConfig struct {
	InstanceID             string
	Region                 string
	PrivateIPAddress       string
	Profile                string
	SharedConfigFiles      []string
	SharedCredentialsFiles []string
	EndpointID             string
	EndpointDNSName        string
	User                   string
	KeyFile                string
	Port                   string
}

func awsEICEConnectorDescribe(params providerapi.ConnectorDescribeParams) (providerapi.ConnectorDescribeResult, error) {
	_, missing := awsEICETargetFromParams(params.Config, params.Target)
	if len(missing) > 0 {
		reason := fmt.Sprintf("aws eice target metadata is missing: %s", strings.Join(missing, ", "))
		return providerapi.ConnectorDescribeResult{
			Capabilities: map[string]providerapi.ConnectorCapability{
				"shell": {Supported: false, Reason: reason},
			},
		}, nil
	}

	commandRuntime := awsEICERuntime(params.Config, params.Target.Config) == "command"
	sdkRuntime := !commandRuntime
	runtimeReason := "aws eice sdk runtime is disabled by configuration"
	if sdkRuntime {
		runtimeReason = ""
	}

	return providerapi.ConnectorDescribeResult{
		Capabilities: map[string]providerapi.ConnectorCapability{
			"shell": {
				Supported: true,
				Preferred: sdkRuntime,
			},
			"exec": {Supported: true},
			"exec_pty": {
				Supported: sdkRuntime,
				Reason:    unsupportedReason(sdkRuntime, runtimeReason),
			},
			"sftp_transport": {Supported: true},
			"upload": {
				Supported: true,
				Requires:  []string{"sftp_transport"},
			},
			"download": {
				Supported: true,
				Requires:  []string{"sftp_transport"},
			},
			"mount": {
				Supported: true,
				Requires:  []string{"sftp_transport"},
			},
			"port_forward_local": {Supported: true},
			"tcp_dial_transport": {
				Supported: sdkRuntime,
				Reason:    unsupportedReason(sdkRuntime, runtimeReason),
			},
			"port_forward_remote": {
				Supported: false,
				Reason:    "aws eice does not support remote port forwarding in the first connector wave",
			},
			"agent_forward": {
				Supported: false,
				Reason:    "aws eice does not provide agent forwarding in the first connector wave",
			},
		},
	}, nil
}

func awsEICEConnectorPrepare(params providerapi.ConnectorPrepareParams) (providerapi.ConnectorPrepareResult, error) {
	target, missing := awsEICETargetFromParams(params.Config, params.Target)
	if len(missing) > 0 {
		return providerapi.ConnectorPrepareResult{
			Supported: false,
			Plan: providerapi.ConnectorPlan{
				Kind: "provider-managed",
				Details: map[string]interface{}{
					"connector": "aws-eice",
					"reason":    fmt.Sprintf("aws eice target metadata is missing: %s", strings.Join(missing, ", ")),
				},
			},
		}, nil
	}

	if awsEICERuntime(params.Config, params.Target.Config) == "command" {
		return awsEICEPrepareCommand(params, target)
	}

	targetPort := awsFirstNonEmpty(awsOptionString(params.Operation.Options, "target_port"), target.Port)
	if targetPort == "" {
		targetPort = "22"
	}
	switch params.Operation.Name {
	case "shell", "exec", "exec_pty", "sftp_transport", "mount", "port_forward_local", "tcp_dial_transport":
		return providerapi.ConnectorPrepareResult{
			Supported: true,
			Plan: providerapi.ConnectorPlan{
				Kind: "provider-managed",
				Details: map[string]interface{}{
					"connector":                          "aws-eice",
					"ssh_runtime":                        "sdk",
					"transport":                          "ssh_transport",
					"operation":                          params.Operation.Name,
					"instance_id":                        target.InstanceID,
					"region":                             target.Region,
					"private_ip_address":                 target.PrivateIPAddress,
					"instance_connect_endpoint_id":       target.EndpointID,
					"instance_connect_endpoint_dns_name": target.EndpointDNSName,
					"profile":                            target.Profile,
					"shared_config_files":                target.SharedConfigFiles,
					"shared_credentials_files":           target.SharedCredentialsFiles,
					"target_port":                        targetPort,
				},
			},
		}, nil
	default:
		return providerapi.ConnectorPrepareResult{
			Supported: false,
			Plan: providerapi.ConnectorPlan{
				Kind: "provider-managed",
				Details: map[string]interface{}{
					"connector": "aws-eice",
					"reason":    fmt.Sprintf("operation %q is not supported by the aws eice connector", params.Operation.Name),
				},
			},
		}, nil
	}
}

func awsEICETargetFromParams(config map[string]interface{}, target providerapi.ConnectorTarget) (awsEICETargetConfig, []string) {
	result := awsEICETargetConfig{
		InstanceID:             awsFirstNonEmpty(target.Meta["instance_id"], providerapi.String(target.Config, "instance_id"), providerapi.String(config, "instance_id")),
		Region:                 awsFirstNonEmpty(target.Meta["region"], providerapi.String(target.Config, "region"), providerapi.String(config, "region")),
		PrivateIPAddress:       awsFirstNonEmpty(target.Meta["private_ip"], providerapi.String(target.Config, "private_ip_address"), providerapi.String(config, "private_ip_address")),
		Profile:                awsFirstNonEmpty(providerapi.String(target.Config, "profile"), providerapi.String(config, "profile")),
		SharedConfigFiles:      providerapi.ExpandPaths(providerapi.StringSlice(config, "shared_config_files")),
		SharedCredentialsFiles: providerapi.ExpandPaths(providerapi.StringSlice(config, "shared_credentials_files")),
		EndpointID:             awsFirstNonEmpty(providerapi.String(target.Config, "instance_connect_endpoint_id"), providerapi.String(config, "instance_connect_endpoint_id")),
		EndpointDNSName:        awsFirstNonEmpty(providerapi.String(target.Config, "instance_connect_endpoint_dns_name"), providerapi.String(config, "instance_connect_endpoint_dns_name")),
		User:                   providerapi.String(target.Config, "user"),
		KeyFile:                providerapi.ExpandPath(providerapi.String(target.Config, "key")),
		Port:                   awsFirstNonEmpty(providerapi.String(target.Config, "port"), "22"),
	}

	var missing []string
	if result.InstanceID == "" {
		missing = append(missing, "instance_id")
	}
	if result.Region == "" {
		missing = append(missing, "region")
	}
	return result, missing
}

func awsEICERuntime(config map[string]interface{}, targetConfig map[string]interface{}) string {
	switch strings.ToLower(strings.TrimSpace(awsFirstNonEmpty(
		providerapi.String(targetConfig, "eice_runtime"),
		providerapi.String(config, "eice_runtime"),
	))) {
	case "", "sdk":
		return "sdk"
	case "command":
		return "command"
	default:
		return "sdk"
	}
}

func awsEICEPrepareCommand(params providerapi.ConnectorPrepareParams, target awsEICETargetConfig) (providerapi.ConnectorPrepareResult, error) {
	opensshCfg := opensshlib.Config{
		SSHPath:      "ssh",
		Host:         target.InstanceID,
		User:         target.User,
		Port:         target.Port,
		IdentityFile: target.KeyFile,
		ExtraOptions: []string{"ProxyCommand=" + eiceconnector.SSHProxyCommand(eiceconnector.ExpandForCommand(eiceconnector.Config{
			InstanceID:             target.InstanceID,
			Region:                 target.Region,
			Profile:                target.Profile,
			EndpointID:             target.EndpointID,
			EndpointDNSName:        target.EndpointDNSName,
			PrivateIPAddress:       target.PrivateIPAddress,
			RemotePort:             target.Port,
			SharedConfigFiles:      target.SharedConfigFiles,
			SharedCredentialsFiles: target.SharedCredentialsFiles,
		}))},
	}

	var plan providerapi.ConnectorPlan
	switch params.Operation.Name {
	case "shell":
		plan = opensshlib.BuildShellPlan(opensshCfg)
	case "exec", "exec_pty":
		plan = opensshlib.BuildExecPlan(opensshCfg, params.Operation)
	case "sftp_transport", "mount":
		plan = opensshlib.BuildSFTPTransportPlan(opensshCfg)
	case "port_forward_local":
		spec, err := awsLocalPortForwardSpecFromOperation(params.Operation)
		if err != nil {
			return providerapi.ConnectorPrepareResult{}, err
		}
		plan = opensshlib.BuildLocalForwardPlan(opensshCfg, spec.ListenHost, spec.ListenPort, spec.TargetHost, spec.TargetPort)
	default:
		return providerapi.ConnectorPrepareResult{
			Supported: false,
			Plan: providerapi.ConnectorPlan{
				Kind: "command",
				Details: map[string]interface{}{
					"connector": "aws-eice",
					"reason":    fmt.Sprintf("operation %q is not supported by the aws eice command runtime", params.Operation.Name),
				},
			},
		}, nil
	}
	plan.Details["connector"] = "aws-eice"
	plan.Details["runtime"] = "command"
	plan.Env = eiceconnector.CommandEnv(eiceconnector.Config{
		SharedConfigFiles:      target.SharedConfigFiles,
		SharedCredentialsFiles: target.SharedCredentialsFiles,
	})
	return providerapi.ConnectorPrepareResult{Supported: true, Plan: plan}, nil
}

func defaultProbeSSMTarget(ctx context.Context, raw map[string]interface{}, target awsSSMTargetConfig) (awsSSMProbeResult, error) {
	cfg, err := loadAWSConfig(ctx, raw, target.Region)
	if err != nil {
		return awsSSMProbeResult{}, err
	}

	client := ssm.NewFromConfig(cfg)
	out, err := client.DescribeInstanceInformation(ctx, awsDescribeInstanceInformationInput(target.InstanceID))
	if err != nil {
		return awsSSMProbeResult{}, fmt.Errorf("describe ssm instance information: %w", err)
	}
	if len(out.InstanceInformationList) == 0 {
		return awsSSMProbeResult{
			Managed:  false,
			Online:   false,
			Platform: target.Platform,
		}, nil
	}

	info := out.InstanceInformationList[0]
	platform := target.Platform
	if platform == "" {
		platform = strings.ToLower(string(info.PlatformType))
	}
	return awsSSMProbeResult{
		Managed:  true,
		Online:   info.PingStatus == ssmtypes.PingStatusOnline || !target.RequireOnline,
		Platform: platform,
	}, nil
}

func awsSSMHealthCheck(ctx context.Context, raw map[string]interface{}, region string) error {
	cfg, err := loadAWSConfig(ctx, raw, region)
	if err != nil {
		return err
	}

	client := ssm.NewFromConfig(cfg)
	_, err = client.DescribeInstanceInformation(ctx, &ssm.DescribeInstanceInformationInput{
		MaxResults: aws.Int32(awsSSMDescribeInstanceInformationMinResults),
	})
	if err != nil {
		return fmt.Errorf("describe ssm instance information: %w", err)
	}
	return nil
}

func unsupportedReason(supported bool, reason string) string {
	if supported {
		return ""
	}
	return reason
}

func awsDescribeInstanceInformationInput(instanceID string) *ssm.DescribeInstanceInformationInput {
	input := &ssm.DescribeInstanceInformationInput{
		MaxResults: aws.Int32(awsSSMDescribeInstanceInformationMinResults),
	}
	if strings.TrimSpace(instanceID) == "" {
		return input
	}

	input.Filters = []ssmtypes.InstanceInformationStringFilter{
		{
			Key:    aws.String("InstanceIds"),
			Values: []string{instanceID},
		},
	}
	return input
}

func awsUnsupportedSSMCapabilities(reason string) providerapi.ConnectorDescribeResult {
	return providerapi.ConnectorDescribeResult{
		Capabilities: map[string]providerapi.ConnectorCapability{
			"shell":    {Supported: false, Reason: reason},
			"exec":     {Supported: false, Reason: reason},
			"exec_pty": {Supported: false, Reason: reason},
		},
	}
}

func awsSSMUnsupportedReason(probe awsSSMProbeResult, fallback string) string {
	if !probe.Managed {
		return "target is not managed by AWS Systems Manager"
	}
	if !probe.Online {
		return "target is managed by AWS Systems Manager but not online"
	}
	return fallback
}

func awsFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func awsBool(raw map[string]interface{}, key string, def bool) bool {
	value := strings.TrimSpace(strings.ToLower(providerapi.String(raw, key)))
	if value == "" {
		return def
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func awsOptionBool(raw map[string]interface{}, key string) bool {
	if raw == nil {
		return false
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		normalized := strings.TrimSpace(strings.ToLower(typed))
		return normalized == "1" || normalized == "true" || normalized == "yes" || normalized == "on"
	default:
		return false
	}
}

func awsOptionString(raw map[string]interface{}, key string) string {
	if raw == nil {
		return ""
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
