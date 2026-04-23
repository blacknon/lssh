package ssmconnector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go"
	"github.com/blacknon/lssh/internal/providerapi"
	"github.com/blacknon/lssh/internal/providerbuiltin"
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
	DocumentName  string
	SessionAction string
	SessionID     string
	Runtime       string
}

type CommandConfig struct {
	BaseConfig
	Command             []string
	CommandDocumentName string
	PollInterval        time.Duration
	Timeout             time.Duration
}

type PortForwardLocalConfig struct {
	BaseConfig
	DocumentName string
	Runtime      string
	ListenHost   string
	ListenPort   string
	TargetHost   string
	TargetPort   string
}

func ShellConfigFromPlan(plan providerapi.ConnectorPlan) (ShellConfig, error) {
	cfg := ShellConfig{BaseConfig: baseConfigFromPlan(plan)}
	cfg.DocumentName = detailString(plan.Details, "document_name")
	cfg.SessionAction = detailString(plan.Details, "session_action")
	cfg.Runtime = detailString(plan.Details, "shell_runtime")
	if cfg.SessionAction == "" {
		cfg.SessionAction = "start"
	}
	if cfg.Runtime == "" {
		cfg.Runtime = "plugin"
	}
	cfg.SessionID = detailString(plan.Details, "session_id")

	switch cfg.SessionAction {
	case "attach":
		if cfg.SessionID == "" || cfg.Region == "" {
			return ShellConfig{}, fmt.Errorf("aws ssm attach plan is missing session_id or region")
		}
	case "detach", "start":
		if cfg.InstanceID == "" || cfg.Region == "" {
			return ShellConfig{}, fmt.Errorf("aws ssm shell plan is missing instance_id or region")
		}
	default:
		return ShellConfig{}, fmt.Errorf("unsupported aws ssm session_action %q", cfg.SessionAction)
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

func PortForwardLocalConfigFromPlan(plan providerapi.ConnectorPlan) (PortForwardLocalConfig, error) {
	cfg := PortForwardLocalConfig{BaseConfig: baseConfigFromPlan(plan)}
	cfg.DocumentName = detailString(plan.Details, "document_name")
	cfg.Runtime = detailString(plan.Details, "port_forward_runtime")
	cfg.ListenHost = detailString(plan.Details, "listen_host")
	cfg.ListenPort = detailString(plan.Details, "listen_port")
	cfg.TargetHost = detailString(plan.Details, "target_host")
	cfg.TargetPort = detailString(plan.Details, "target_port")
	if cfg.Runtime == "" {
		cfg.Runtime = "plugin"
	}
	if cfg.InstanceID == "" || cfg.Region == "" {
		return PortForwardLocalConfig{}, fmt.Errorf("aws ssm local port forward plan is missing instance_id or region")
	}
	if cfg.ListenPort == "" || cfg.TargetHost == "" || cfg.TargetPort == "" {
		return PortForwardLocalConfig{}, fmt.Errorf("aws ssm local port forward plan is missing listen_port, target_host, or target_port")
	}
	return cfg, nil
}

func StartShell(ctx context.Context, cfg ShellConfig) error {
	return StartShellWithIO(ctx, cfg, os.Stdin, os.Stdout, os.Stderr)
}

func StartShellWithIO(ctx context.Context, cfg ShellConfig, stdin io.Reader, stdout, stderr io.Writer) error {
	if cfg.SessionAction == "detach" {
		return fmt.Errorf("detached shell sessions must use StartDetachedShell")
	}
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
	args := []string{}
	switch cfg.SessionAction {
	case "attach":
		args = []string{
			"ssm", "resume-session",
			"--session-id", cfg.SessionID,
			"--region", cfg.Region,
		}
	default:
		args = []string{
			"ssm", "start-session",
			"--target", cfg.InstanceID,
			"--region", cfg.Region,
		}
	}
	if cfg.Profile != "" {
		args = append(args, "--profile", cfg.Profile)
	}
	if cfg.SessionAction != "attach" && cfg.DocumentName != "" {
		args = append(args, "--document-name", cfg.DocumentName)
	}

	cmd := exec.CommandContext(ctx, "aws", args...)
	cmd.Env = append(os.Environ(), shellEnvironment(cfg)...)
	return cmd
}

func StartDetachedShell(ctx context.Context, cfg ShellConfig) (string, error) {
	if cfg.SessionAction != "detach" {
		return "", fmt.Errorf("detached shell requires session_action=detach")
	}

	awsCfg, err := loadAWSConfig(ctx, cfg.BaseConfig)
	if err != nil {
		return "", err
	}

	input := &ssm.StartSessionInput{
		Target: aws.String(cfg.InstanceID),
	}
	if cfg.DocumentName != "" {
		input.DocumentName = aws.String(cfg.DocumentName)
	}

	client := ssm.NewFromConfig(awsCfg)
	out, err := client.StartSession(ctx, input)
	if err != nil {
		return "", fmt.Errorf("start detached ssm session: %w", err)
	}
	sessionID := aws.ToString(out.SessionId)
	if sessionID == "" {
		return "", fmt.Errorf("start detached ssm session returned empty session id")
	}
	return sessionID, nil
}

func StartLocalPortForward(ctx context.Context, cfg PortForwardLocalConfig) error {
	cmd := BuildStartPortForwardCommand(ctx, cfg)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start aws ssm local port forward: %w", err)
	}
	if os.Getenv("_LSSH_DAEMON") == "1" {
		notifyParentReady()
	}
	if err := waitCommandWithInterrupt(cmd); err != nil {
		return fmt.Errorf("run aws ssm local port forward: %w", err)
	}
	return nil
}

func BuildStartPortForwardCommand(ctx context.Context, cfg PortForwardLocalConfig) *exec.Cmd {
	args := []string{
		"ssm", "start-session",
		"--target", cfg.InstanceID,
		"--region", cfg.Region,
	}
	if cfg.Profile != "" {
		args = append(args, "--profile", cfg.Profile)
	}

	documentName := cfg.DocumentName
	if documentName == "" {
		documentName = "AWS-StartPortForwardingSessionToRemoteHost"
	}
	args = append(args, "--document-name", documentName)

	parameters, err := json.Marshal(map[string][]string{
		"host":            {cfg.TargetHost},
		"portNumber":      {cfg.TargetPort},
		"localPortNumber": {cfg.ListenPort},
	})
	if err == nil {
		args = append(args, "--parameters", string(parameters))
	}

	cmd := exec.CommandContext(ctx, "aws", args...)
	cmd.Env = append(os.Environ(), portForwardEnvironment(cfg)...)
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
				if shouldRetryGetCommandInvocation(err) {
					continue
				}
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

func shouldRetryGetCommandInvocation(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}

	switch apiErr.ErrorCode() {
	case "InvocationDoesNotExist":
		return true
	default:
		return false
	}
}

func loadAWSConfig(ctx context.Context, cfg BaseConfig) (aws.Config, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if cfg.Profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(cfg.Profile))
	}
	if files := providerbuiltin.ExpandPaths(cfg.SharedConfigFiles); len(files) > 0 {
		opts = append(opts, awsconfig.WithSharedConfigFiles(files))
	}
	if files := providerbuiltin.ExpandPaths(cfg.SharedCredentialsFiles); len(files) > 0 {
		opts = append(opts, awsconfig.WithSharedCredentialsFiles(files))
	}
	return awsconfig.LoadDefaultConfig(ctx, opts...)
}

func shellEnvironment(cfg ShellConfig) []string {
	return baseEnvironment(cfg.BaseConfig)
}

func portForwardEnvironment(cfg PortForwardLocalConfig) []string {
	return baseEnvironment(cfg.BaseConfig)
}

func baseEnvironment(cfg BaseConfig) []string {
	env := []string{}
	sharedConfigFiles := providerbuiltin.ExpandPaths(cfg.SharedConfigFiles)
	sharedCredentialsFiles := providerbuiltin.ExpandPaths(cfg.SharedCredentialsFiles)
	if len(sharedConfigFiles) > 0 && sharedConfigFiles[0] != "" {
		env = append(env, "AWS_CONFIG_FILE="+sharedConfigFiles[0])
	}
	if len(sharedCredentialsFiles) > 0 && sharedCredentialsFiles[0] != "" {
		env = append(env, "AWS_SHARED_CREDENTIALS_FILE="+sharedCredentialsFiles[0])
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

func notifyParentReady() {
	if os.Getenv("_LSSH_DAEMON") != "1" {
		return
	}

	f := os.NewFile(uintptr(3), "lssh_ready")
	if f == nil {
		return
	}
	defer f.Close()

	_, _ = f.Write([]byte("OK\n"))
}

func waitCommandWithInterrupt(cmd *exec.Cmd) error {
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	defer signal.Stop(signalCh)

	select {
	case err := <-waitCh:
		return err
	case sig := <-signalCh:
		if cmd.Process != nil {
			_ = cmd.Process.Signal(sig)
		}
		err := <-waitCh
		if isInterruptExit(err) {
			return nil
		}
		return err
	}
}

func isInterruptExit(err error) bool {
	if err == nil {
		return false
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	if exitErr.ProcessState == nil {
		return false
	}
	return exitErr.ProcessState.ExitCode() == 130
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
