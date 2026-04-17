package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/blacknon/lssh/internal/providerapi"
	"github.com/blacknon/lssh/internal/providerbuiltin"
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

func awsConnectorDescribe(params providerapi.ConnectorDescribeParams) (providerapi.ConnectorDescribeResult, error) {
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
				Supported: false,
				Reason:    "aws ssm port forwarding is not implemented in the first connector wave",
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
			"profile":                  providerbuiltin.String(params.Config, "profile"),
			"shared_config_files":      providerbuiltin.StringSlice(params.Config, "shared_config_files"),
			"shared_credentials_files": providerbuiltin.StringSlice(params.Config, "shared_credentials_files"),
			"poll_interval_sec":        2,
			"timeout_sec":              60,
			"operation":                params.Operation.Name,
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
		plan.Details["reason"] = awsSSMUnsupportedReason(probe, "aws ssm target is not online")
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
		providerbuiltin.String(target.Config, "instance_id"),
		providerbuiltin.String(config, "instance_id"),
	)
	region := awsFirstNonEmpty(
		target.Meta["region"],
		providerbuiltin.String(target.Config, "region"),
		providerbuiltin.String(config, "region"),
	)
	platform := strings.ToLower(awsFirstNonEmpty(
		target.Meta["platform"],
		providerbuiltin.String(target.Config, "platform"),
		providerbuiltin.String(config, "platform"),
	))
	result := awsSSMTargetConfig{
		InstanceID:                 instanceID,
		Region:                     region,
		Platform:                   platform,
		ShellDocumentName:          providerbuiltin.String(config, "ssm_shell_document"),
		InteractiveCommandDocument: providerbuiltin.String(config, "ssm_interactive_command_document"),
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

func defaultProbeSSMTarget(ctx context.Context, raw map[string]interface{}, target awsSSMTargetConfig) (awsSSMProbeResult, error) {
	cfg, err := loadAWSConfig(ctx, raw, target.Region)
	if err != nil {
		return awsSSMProbeResult{}, err
	}

	client := ssm.NewFromConfig(cfg)
	out, err := client.DescribeInstanceInformation(ctx, &ssm.DescribeInstanceInformationInput{
		Filters: []ssmtypes.InstanceInformationStringFilter{
			{
				Key:    aws.String("InstanceIds"),
				Values: []string{target.InstanceID},
			},
		},
		MaxResults: aws.Int32(1),
	})
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
		MaxResults: aws.Int32(1),
	})
	if err != nil {
		return fmt.Errorf("describe ssm instance information: %w", err)
	}
	return nil
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
	value := strings.TrimSpace(strings.ToLower(providerbuiltin.String(raw, key)))
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
