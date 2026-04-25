package ssmconnector

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/smithy-go"
	"github.com/blacknon/lssh/providerapi"
)

func TestBuildStartSessionCommand(t *testing.T) {
	cmd := BuildStartSessionCommand(context.Background(), ShellConfig{
		BaseConfig: BaseConfig{
			InstanceID:             "i-0123456789abcdef0",
			Region:                 "ap-northeast-1",
			Profile:                "default",
			SharedConfigFiles:      []string{"/tmp/aws-config"},
			SharedCredentialsFiles: []string{"/tmp/aws-credentials"},
		},
		DocumentName: "AWS-StartInteractiveCommand",
	})

	if got, want := filepath.Base(cmd.Path), "aws"; got != want {
		t.Fatalf("cmd.Path = %q, want %q", got, want)
	}

	args := cmd.Args
	wantArgs := []string{
		"aws", "ssm", "start-session",
		"--target", "i-0123456789abcdef0",
		"--region", "ap-northeast-1",
		"--profile", "default",
		"--document-name", "AWS-StartInteractiveCommand",
	}
	if len(args) != len(wantArgs) {
		t.Fatalf("len(cmd.Args) = %d, want %d (%v)", len(args), len(wantArgs), args)
	}
	for i := range wantArgs {
		if args[i] != wantArgs[i] {
			t.Fatalf("cmd.Args[%d] = %q, want %q", i, args[i], wantArgs[i])
		}
	}

	env := map[string]bool{}
	for _, item := range cmd.Env {
		env[item] = true
	}
	if !env["AWS_CONFIG_FILE=/tmp/aws-config"] {
		t.Fatal("AWS_CONFIG_FILE is not set in command env")
	}
	if !env["AWS_SHARED_CREDENTIALS_FILE=/tmp/aws-credentials"] {
		t.Fatal("AWS_SHARED_CREDENTIALS_FILE is not set in command env")
	}
}

func TestBuildResumeSessionCommand(t *testing.T) {
	cmd := BuildStartSessionCommand(context.Background(), ShellConfig{
		BaseConfig: BaseConfig{
			Region: "ap-northeast-1",
		},
		SessionAction: "attach",
		SessionID:     "session-123",
	})

	wantArgs := []string{
		"aws", "ssm", "resume-session",
		"--session-id", "session-123",
		"--region", "ap-northeast-1",
	}
	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("len(cmd.Args) = %d, want %d (%v)", len(cmd.Args), len(wantArgs), cmd.Args)
	}
	for i := range wantArgs {
		if cmd.Args[i] != wantArgs[i] {
			t.Fatalf("cmd.Args[%d] = %q, want %q", i, cmd.Args[i], wantArgs[i])
		}
	}
}

func TestBuildStartSessionCommandInteractiveCommand(t *testing.T) {
	cmd := BuildStartSessionCommand(context.Background(), ShellConfig{
		BaseConfig: BaseConfig{
			InstanceID: "i-0123456789abcdef0",
			Region:     "ap-northeast-1",
		},
		DocumentName: "AWS-StartInteractiveCommand",
		Command:      []string{"stty", "size"},
	})

	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "--document-name AWS-StartInteractiveCommand") {
		t.Fatalf("cmd.Args = %v, want document name", cmd.Args)
	}
	if !strings.Contains(args, `--parameters {"command":["stty size"]}`) {
		t.Fatalf("cmd.Args = %v, want interactive command parameters", cmd.Args)
	}
}

func TestBuildStartSessionCommandExpandsSharedConfigPaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}

	cmd := BuildStartSessionCommand(context.Background(), ShellConfig{
		BaseConfig: BaseConfig{
			InstanceID:             "i-0123456789abcdef0",
			Region:                 "ap-northeast-1",
			SharedConfigFiles:      []string{"~/.aws/config"},
			SharedCredentialsFiles: []string{"~/.aws/credentials"},
		},
	})

	env := map[string]bool{}
	for _, item := range cmd.Env {
		env[item] = true
	}
	if !env["AWS_CONFIG_FILE="+filepath.Join(home, ".aws", "config")] {
		t.Fatal("AWS_CONFIG_FILE is not expanded in command env")
	}
	if !env["AWS_SHARED_CREDENTIALS_FILE="+filepath.Join(home, ".aws", "credentials")] {
		t.Fatal("AWS_SHARED_CREDENTIALS_FILE is not expanded in command env")
	}
}

func TestBuildStartPortForwardCommand(t *testing.T) {
	cmd := BuildStartPortForwardCommand(context.Background(), PortForwardLocalConfig{
		BaseConfig: BaseConfig{
			InstanceID:             "i-0123456789abcdef0",
			Region:                 "ap-northeast-1",
			Profile:                "default",
			SharedConfigFiles:      []string{"/tmp/aws-config"},
			SharedCredentialsFiles: []string{"/tmp/aws-credentials"},
		},
		ListenHost: "localhost",
		ListenPort: "15432",
		TargetHost: "db.internal",
		TargetPort: "5432",
	})

	wantArgs := []string{
		"aws", "ssm", "start-session",
		"--target", "i-0123456789abcdef0",
		"--region", "ap-northeast-1",
		"--profile", "default",
		"--document-name", "AWS-StartPortForwardingSessionToRemoteHost",
		"--parameters", `{"host":["db.internal"],"localPortNumber":["15432"],"portNumber":["5432"]}`,
	}
	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("len(cmd.Args) = %d, want %d (%v)", len(cmd.Args), len(wantArgs), cmd.Args)
	}
	for i := range wantArgs {
		if cmd.Args[i] != wantArgs[i] {
			t.Fatalf("cmd.Args[%d] = %q, want %q", i, cmd.Args[i], wantArgs[i])
		}
	}

	env := map[string]bool{}
	for _, item := range cmd.Env {
		env[item] = true
	}
	if !env["AWS_CONFIG_FILE=/tmp/aws-config"] {
		t.Fatal("AWS_CONFIG_FILE is not set in command env")
	}
	if !env["AWS_SHARED_CREDENTIALS_FILE=/tmp/aws-credentials"] {
		t.Fatal("AWS_SHARED_CREDENTIALS_FILE is not set in command env")
	}
}

func TestPortForwardDialConfigFromPlan(t *testing.T) {
	cfg, err := PortForwardDialConfigFromPlan(providerapi.ConnectorPlan{
		Details: map[string]interface{}{
			"instance_id":          "i-0123456789abcdef0",
			"region":               "ap-northeast-1",
			"port_forward_runtime": "plugin",
			"target_host":          "db.internal",
			"target_port":          "5432",
		},
	})
	if err != nil {
		t.Fatalf("PortForwardDialConfigFromPlan() error = %v", err)
	}
	if cfg.TargetHost != "db.internal" {
		t.Fatalf("cfg.TargetHost = %q, want db.internal", cfg.TargetHost)
	}
	if cfg.TargetPort != "5432" {
		t.Fatalf("cfg.TargetPort = %q, want 5432", cfg.TargetPort)
	}
}

func TestIsInterruptExit(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 130")
	err := cmd.Run()
	if err == nil {
		t.Fatal("cmd.Run() error = nil, want non-nil")
	}
	if !isInterruptExit(err) {
		t.Fatal("isInterruptExit(exit 130) = false, want true")
	}
	if isInterruptExit(errors.New("other")) {
		t.Fatal("isInterruptExit(other error) = true, want false")
	}
}

func TestShouldRetryGetCommandInvocation(t *testing.T) {
	retryErr := &smithy.GenericAPIError{Code: "InvocationDoesNotExist", Message: "not ready"}
	if !shouldRetryGetCommandInvocation(retryErr) {
		t.Fatal("shouldRetryGetCommandInvocation() = false, want true")
	}

	otherErr := &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "denied"}
	if shouldRetryGetCommandInvocation(otherErr) {
		t.Fatal("shouldRetryGetCommandInvocation() = true, want false")
	}
}
