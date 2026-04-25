package psrplib

import (
	"fmt"
	"strings"

	"github.com/blacknon/lssh/providerapi"
)

const (
	envPowerShellHost              = "LSSH_PSRP_HOST"
	envPowerShellUser              = "LSSH_PSRP_USER"
	envPowerShellPassword          = "LSSH_PSRP_PASSWORD"
	envPowerShellPort              = "LSSH_PSRP_PORT"
	envPowerShellUseSSL            = "LSSH_PSRP_USE_SSL"
	envPowerShellInsecure          = "LSSH_PSRP_INSECURE"
	envPowerShellAuthentication    = "LSSH_PSRP_AUTHENTICATION"
	envPowerShellConfigurationName = "LSSH_PSRP_CONFIGURATION_NAME"
	envPowerShellOperationTimeout  = "LSSH_PSRP_OPERATION_TIMEOUT_SEC"
	envPowerShellCommand           = "LSSH_PSRP_COMMAND"
)

func BuildShellPlan(cfg Config) providerapi.ConnectorPlan {
	args := append([]string{}, cfg.PowerShellOptions...)
	args = append(args, "-NoLogo", "-NoProfile", "-NoExit", "-Command", buildShellScript())

	return providerapi.ConnectorPlan{
		Kind:    "command",
		Program: cfg.PowerShellPath,
		Args:    args,
		Env:     buildBaseEnv(cfg, ""),
		Details: map[string]interface{}{
			"connector": "psrp",
			"operation": "shell",
			"transport": "shell_transport",
		},
	}
}

func BuildExecPlan(cfg Config, op providerapi.ConnectorOperation) providerapi.ConnectorPlan {
	commandLine := strings.TrimSpace(strings.Join(op.Command, " "))
	args := append([]string{}, cfg.PowerShellOptions...)
	args = append(args, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", buildExecScript())

	return providerapi.ConnectorPlan{
		Kind:    "command",
		Program: cfg.PowerShellPath,
		Args:    args,
		Env:     buildBaseEnv(cfg, commandLine),
		Details: map[string]interface{}{
			"connector":    "psrp",
			"operation":    op.Name,
			"transport":    "exec_transport",
			"command":      append([]string(nil), op.Command...),
			"command_line": commandLine,
		},
	}
}

func buildBaseEnv(cfg Config, commandLine string) map[string]string {
	env := map[string]string{
		envPowerShellHost:             cfg.Host,
		envPowerShellUser:             cfg.User,
		envPowerShellPassword:         cfg.Password,
		envPowerShellPort:             fmt.Sprintf("%d", cfg.Port),
		envPowerShellUseSSL:           fmt.Sprintf("%t", cfg.HTTPS),
		envPowerShellInsecure:         fmt.Sprintf("%t", cfg.Insecure),
		envPowerShellAuthentication:   cfg.Authentication,
		envPowerShellOperationTimeout: fmt.Sprintf("%d", cfg.OperationTimeoutSec),
	}

	if cfg.ConfigurationName != "" {
		env[envPowerShellConfigurationName] = cfg.ConfigurationName
	}
	if commandLine != "" {
		env[envPowerShellCommand] = commandLine
	}

	return env
}

func buildShellScript() string {
	return strings.Join([]string{
		"$ErrorActionPreference = 'Stop'",
		"$pass = ConvertTo-SecureString $env:LSSH_PSRP_PASSWORD -AsPlainText -Force",
		"$cred = [System.Management.Automation.PSCredential]::new($env:LSSH_PSRP_USER, $pass)",
		"$sessionOptionArgs = @{}",
		"if ([System.Convert]::ToBoolean($env:LSSH_PSRP_INSECURE)) { $sessionOptionArgs.SkipCACheck = $true; $sessionOptionArgs.SkipCNCheck = $true }",
		"if ($env:LSSH_PSRP_OPERATION_TIMEOUT_SEC) { $sessionOptionArgs.OperationTimeout = New-TimeSpan -Seconds ([int]$env:LSSH_PSRP_OPERATION_TIMEOUT_SEC) }",
		"$sessionOptions = New-PSSessionOption @sessionOptionArgs",
		"$connectionArgs = @{ ComputerName = $env:LSSH_PSRP_HOST; Port = [int]$env:LSSH_PSRP_PORT; Credential = $cred; Authentication = $env:LSSH_PSRP_AUTHENTICATION; SessionOption = $sessionOptions }",
		"if ([System.Convert]::ToBoolean($env:LSSH_PSRP_USE_SSL)) { $connectionArgs.UseSSL = $true }",
		"if ($env:LSSH_PSRP_CONFIGURATION_NAME) { $connectionArgs.ConfigurationName = $env:LSSH_PSRP_CONFIGURATION_NAME }",
		"Enter-PSSession @connectionArgs",
	}, "; ")
}

func buildExecScript() string {
	return strings.Join([]string{
		"$ErrorActionPreference = 'Stop'",
		"$pass = ConvertTo-SecureString $env:LSSH_PSRP_PASSWORD -AsPlainText -Force",
		"$cred = [System.Management.Automation.PSCredential]::new($env:LSSH_PSRP_USER, $pass)",
		"$sessionOptionArgs = @{}",
		"if ([System.Convert]::ToBoolean($env:LSSH_PSRP_INSECURE)) { $sessionOptionArgs.SkipCACheck = $true; $sessionOptionArgs.SkipCNCheck = $true }",
		"if ($env:LSSH_PSRP_OPERATION_TIMEOUT_SEC) { $sessionOptionArgs.OperationTimeout = New-TimeSpan -Seconds ([int]$env:LSSH_PSRP_OPERATION_TIMEOUT_SEC) }",
		"$sessionOptions = New-PSSessionOption @sessionOptionArgs",
		"$connectionArgs = @{ ComputerName = $env:LSSH_PSRP_HOST; Port = [int]$env:LSSH_PSRP_PORT; Credential = $cred; Authentication = $env:LSSH_PSRP_AUTHENTICATION; SessionOption = $sessionOptions }",
		"if ([System.Convert]::ToBoolean($env:LSSH_PSRP_USE_SSL)) { $connectionArgs.UseSSL = $true }",
		"if ($env:LSSH_PSRP_CONFIGURATION_NAME) { $connectionArgs.ConfigurationName = $env:LSSH_PSRP_CONFIGURATION_NAME }",
		"try {",
		"  Invoke-Command @connectionArgs -ScriptBlock {",
		"    param([string]$RemoteCommand)",
		"    $ErrorActionPreference = 'Stop'",
		"    & ([scriptblock]::Create($RemoteCommand))",
		"    if ($LASTEXITCODE -is [int] -and $LASTEXITCODE -ne 0) { throw \"remote command exited with status $LASTEXITCODE\" }",
		"  } -ArgumentList $env:LSSH_PSRP_COMMAND",
		"  exit 0",
		"} catch {",
		"  Write-Error $_",
		"  exit 1",
		"}",
	}, "; ")
}
