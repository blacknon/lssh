package psrplib

import (
	"os"
	"strings"
	"testing"

	"github.com/blacknon/lssh/providerapi"
)

func TestConfigFromMapsDefaults(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	cfg, err := ConfigFromMaps(map[string]interface{}{
		"pwsh_path": exe,
		"https":     "true",
	}, map[string]interface{}{
		"addr": "windows.local",
		"user": "Administrator",
		"pass": "secret",
	})
	if err != nil {
		t.Fatalf("ConfigFromMaps() error = %v", err)
	}

	if cfg.Port != 5986 {
		t.Fatalf("Port = %d, want 5986", cfg.Port)
	}
	if cfg.Authentication != "Negotiate" {
		t.Fatalf("Authentication = %q, want Negotiate", cfg.Authentication)
	}
}

func TestBuildShellPlan(t *testing.T) {
	cfg := Config{
		PowerShellPath: "/usr/bin/pwsh",
		Host:           "windows.local",
		User:           "Administrator",
		Password:       "secret",
		Port:           5986,
		HTTPS:          true,
		Insecure:       true,
		Authentication: "Basic",
	}

	plan := BuildShellPlan(cfg)
	if plan.Kind != "command" {
		t.Fatalf("Kind = %q, want command", plan.Kind)
	}
	if plan.Program != cfg.PowerShellPath {
		t.Fatalf("Program = %q, want %q", plan.Program, cfg.PowerShellPath)
	}
	if !strings.Contains(strings.Join(plan.Args, " "), "Enter-PSSession") {
		t.Fatalf("Args = %v, want Enter-PSSession", plan.Args)
	}
	if plan.Env[envPowerShellAuthentication] != "Basic" {
		t.Fatalf("Auth env = %q, want Basic", plan.Env[envPowerShellAuthentication])
	}
}

func TestBuildExecPlan(t *testing.T) {
	cfg := Config{
		PowerShellPath: "/usr/bin/pwsh",
		Host:           "windows.local",
		User:           "Administrator",
		Password:       "secret",
		Port:           5985,
		Authentication: "Negotiate",
	}

	plan := BuildExecPlan(cfg, providerapi.ConnectorOperation{
		Name:    "exec",
		Command: []string{"Get-Location"},
	})

	if !strings.Contains(strings.Join(plan.Args, " "), "Invoke-Command") {
		t.Fatalf("Args = %v, want Invoke-Command", plan.Args)
	}
	if got := plan.Env[envPowerShellCommand]; got != "Get-Location" {
		t.Fatalf("command env = %q, want Get-Location", got)
	}
}

func TestResolveRuntime(t *testing.T) {
	runtime := ResolveRuntime(map[string]interface{}{
		"shell_runtime": "library",
	}, map[string]interface{}{
		"exec_runtime": "command",
	})

	if runtime.Shell != RuntimeLibrary {
		t.Fatalf("Shell runtime = %q, want %q", runtime.Shell, RuntimeLibrary)
	}
	if runtime.Exec != RuntimeCommand {
		t.Fatalf("Exec runtime = %q, want %q", runtime.Exec, RuntimeCommand)
	}
}

func TestBuildLibraryShellPlan(t *testing.T) {
	cfg := Config{
		Host:     "windows.local",
		User:     "Administrator",
		Password: "secret",
		Port:     5986,
		HTTPS:    true,
	}

	plan, err := BuildLibraryShellPlan(cfg)
	if err != nil {
		t.Fatalf("BuildLibraryShellPlan() error = %v", err)
	}
	if plan.Kind != "command" {
		t.Fatalf("Kind = %q, want command", plan.Kind)
	}
	if len(plan.Args) < 2 || plan.Args[0] != helperCommand || plan.Args[1] != helperShell {
		t.Fatalf("Args = %v, want helper shell invocation", plan.Args)
	}
}
