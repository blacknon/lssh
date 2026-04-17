package ssmconnector

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/blacknon/lssh/internal/providerapi"
	"github.com/kballard/go-shellquote"
)

type BaseConfig struct {
	InstanceID             string
	Region                 string
	Platform               string
	Profile                string
	SharedConfigFiles      []string
	SharedCredentialsFiles []string
}

type ShellConfig struct {
	BaseConfig
	DocumentName string
}

type CommandConfig struct {
	BaseConfig
	Command             []string
	CommandDocumentName string
	PollInterval        time.Duration
	Timeout             time.Duration
}

func ShellConfigFromPlan(plan providerapi.ConnectorPlan) (ShellConfig, error) {
	cfg := ShellConfig{BaseConfig: baseConfigFromPlan(plan)}
	cfg.DocumentName = detailString(plan.Details, "document_name")
	if cfg.InstanceID == "" || cfg.Region == "" {
		return ShellConfig{}, fmt.Errorf("aws ssm shell plan is missing instance_id or region")
	}
	return cfg, nil
}

func CommandConfigFromPlan(plan providerapi.ConnectorPlan) (CommandConfig, error) {
	cfg := CommandConfig{BaseConfig: baseConfigFromPlan(plan)}
	cfg.Command = detailStringSlice(plan.Details, "command")
	cfg.CommandDocumentName = detailString(plan.Details, "command_document_name")
	cfg.PollInterval = detailDuration(plan.Details, "poll_interval_sec", 2*time.Second)
	cfg.Timeout = detailDuration(plan.Details, "timeout_sec", 60*time.Second)
	if cfg.InstanceID == "" || cfg.Region == "" {
		return CommandConfig{}, fmt.Errorf("aws ssm command plan is missing instance_id or region")
	}
	return cfg, nil
}

func StartShell(ctx context.Context, cfg ShellConfig) error {
	return StartShellWithIO(ctx, cfg, os.Stdin, os.Stdout, os.Stderr)
}

func StartShellWithIO(ctx context.Context, cfg ShellConfig, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := BuildStartSessionCommand(ctx, cfg)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run aws ssm start-session: %w", err)
	}
	return nil
}

func BuildStartSessionCommand(ctx context.Context, cfg ShellConfig) *exec.Cmd {
	args := []string{
		"ssm", "start-session",
		"--target", cfg.InstanceID,
		"--region", cfg.Region,
	}
	if cfg.Profile != "" {
		args = append(args, "--profile", cfg.Profile)
	}
	if cfg.DocumentName != "" {
		args = append(args, "--document-name", cfg.DocumentName)
	}

	cmd := exec.CommandContext(ctx, "aws", args...)
	cmd.Env = append(os.Environ(), shellEnvironment(cfg)...)
	return cmd
}

func RunCommand(ctx context.Context, cfg CommandConfig, stdout, stderr io.Writer) (int, error) {
	awsCfg, err := loadAWSConfig(ctx, cfg.BaseConfig)
	if err != nil {
		return 0, err
	}

	client := ssm.NewFromConfig(awsCfg)
	documentName := cfg.CommandDocumentName
	if documentName == "" {
		if strings.EqualFold(cfg.Platform, "windows") {
			documentName = "AWS-RunPowerShellScript"
		} else {
			documentName = "AWS-RunShellScript"
		}
	}

	commandText := strings.Join(cfg.Command, " ")
	if !strings.EqualFold(cfg.Platform, "windows") {
		commandText = shellquote.Join(cfg.Command...)
	}

	sendOut, err := client.SendCommand(ctx, &ssm.SendCommandInput{
		InstanceIds:  []string{cfg.InstanceID},
		DocumentName: aws.String(documentName),
		Parameters: map[string][]string{
			"commands": {commandText},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("send ssm command: %w", err)
	}

	commandID := aws.ToString(sendOut.Command.CommandId)
	if commandID == "" {
		return 0, fmt.Errorf("send ssm command returned empty command id")
	}

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	timeoutCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return 0, fmt.Errorf("wait ssm command invocation: %w", timeoutCtx.Err())
		case <-ticker.C:
			out, err := client.GetCommandInvocation(timeoutCtx, &ssm.GetCommandInvocationInput{
				CommandId:  aws.String(commandID),
				InstanceId: aws.String(cfg.InstanceID),
			})
			if err != nil {
				return 0, fmt.Errorf("get ssm command invocation: %w", err)
			}
			switch out.Status {
			case ssmtypes.CommandInvocationStatusPending, ssmtypes.CommandInvocationStatusInProgress, ssmtypes.CommandInvocationStatusDelayed, ssmtypes.CommandInvocationStatusCancelling:
				continue
			default:
				if stdout != nil && out.StandardOutputContent != nil {
					_, _ = io.WriteString(stdout, aws.ToString(out.StandardOutputContent))
				}
				if stderr != nil && out.StandardErrorContent != nil {
					_, _ = io.WriteString(stderr, aws.ToString(out.StandardErrorContent))
				}
				return int(out.ResponseCode), nil
			}
		}
	}
}

func loadAWSConfig(ctx context.Context, cfg BaseConfig) (aws.Config, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if cfg.Profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(cfg.Profile))
	}
	if len(cfg.SharedConfigFiles) > 0 {
		opts = append(opts, awsconfig.WithSharedConfigFiles(cfg.SharedConfigFiles))
	}
	if len(cfg.SharedCredentialsFiles) > 0 {
		opts = append(opts, awsconfig.WithSharedCredentialsFiles(cfg.SharedCredentialsFiles))
	}
	return awsconfig.LoadDefaultConfig(ctx, opts...)
}

func shellEnvironment(cfg ShellConfig) []string {
	env := []string{}
	if len(cfg.SharedConfigFiles) > 0 && cfg.SharedConfigFiles[0] != "" {
		env = append(env, "AWS_CONFIG_FILE="+cfg.SharedConfigFiles[0])
	}
	if len(cfg.SharedCredentialsFiles) > 0 && cfg.SharedCredentialsFiles[0] != "" {
		env = append(env, "AWS_SHARED_CREDENTIALS_FILE="+cfg.SharedCredentialsFiles[0])
	}
	return env
}

func baseConfigFromPlan(plan providerapi.ConnectorPlan) BaseConfig {
	return BaseConfig{
		InstanceID:             detailString(plan.Details, "instance_id"),
		Region:                 detailString(plan.Details, "region"),
		Platform:               detailString(plan.Details, "platform"),
		Profile:                detailString(plan.Details, "profile"),
		SharedConfigFiles:      detailStringSlice(plan.Details, "shared_config_files"),
		SharedCredentialsFiles: detailStringSlice(plan.Details, "shared_credentials_files"),
	}
}

func detailString(details map[string]interface{}, key string) string {
	if details == nil {
		return ""
	}
	if value, ok := details[key]; ok && value != nil {
		return fmt.Sprint(value)
	}
	return ""
}

func detailStringSlice(details map[string]interface{}, key string) []string {
	if details == nil {
		return nil
	}
	raw, ok := details[key]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if item == nil {
				continue
			}
			result = append(result, fmt.Sprint(item))
		}
		return result
	default:
		return []string{fmt.Sprint(typed)}
	}
}

func detailDuration(details map[string]interface{}, key string, def time.Duration) time.Duration {
	if details == nil {
		return def
	}
	raw, ok := details[key]
	if !ok || raw == nil {
		return def
	}
	switch typed := raw.(type) {
	case int:
		return time.Duration(typed) * time.Second
	case int64:
		return time.Duration(typed) * time.Second
	case float64:
		return time.Duration(typed) * time.Second
	case string:
		if typed == "" {
			return def
		}
		if d, err := time.ParseDuration(typed); err == nil {
			return d
		}
	}
	return def
}
