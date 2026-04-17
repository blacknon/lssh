package ssmconnector

import (
	"context"
	"path/filepath"
	"testing"
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
