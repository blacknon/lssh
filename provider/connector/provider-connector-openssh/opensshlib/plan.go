package opensshlib

import (
	"fmt"
	"net"
	"strings"

	"github.com/blacknon/lssh/providerapi"
)

type Config struct {
	SSHPath               string
	Host                  string
	User                  string
	Port                  string
	IdentityFile          string
	SSHConfigFile         string
	UserKnownHostsFile    string
	StrictHostKeyChecking string
	BatchMode             bool
	ForwardAgent          bool
	ExtraOptions          []string
}

func ConfigFromMaps(provider map[string]interface{}, target map[string]interface{}) (Config, error) {
	cfg := Config{
		SSHPath:               firstNonEmpty(providerapi.String(target, "ssh_path"), providerapi.String(provider, "ssh_path"), "ssh"),
		Host:                  firstNonEmpty(providerapi.String(target, "addr"), providerapi.String(provider, "addr")),
		User:                  firstNonEmpty(providerapi.String(target, "user"), providerapi.String(provider, "user")),
		Port:                  firstNonEmpty(providerapi.String(target, "port"), providerapi.String(provider, "port")),
		IdentityFile:          firstNonEmpty(providerapi.String(target, "key"), providerapi.String(target, "identity_file"), providerapi.String(provider, "identity_file")),
		SSHConfigFile:         firstNonEmpty(providerapi.String(target, "ssh_config_file"), providerapi.String(provider, "ssh_config_file")),
		UserKnownHostsFile:    firstNonEmpty(providerapi.String(target, "user_known_hosts_file"), providerapi.String(provider, "user_known_hosts_file")),
		StrictHostKeyChecking: firstNonEmpty(providerapi.String(target, "strict_host_key_checking"), providerapi.String(provider, "strict_host_key_checking")),
		BatchMode:             parseBool(target, "batch_mode") || parseBool(provider, "batch_mode"),
		ForwardAgent:          parseBool(target, "forward_agent") || parseBool(provider, "forward_agent"),
		ExtraOptions:          firstStringSlice(providerapi.StringSlice(target, "ssh_options"), providerapi.StringSlice(provider, "ssh_options")),
	}

	if strings.TrimSpace(cfg.Host) == "" {
		return Config{}, fmt.Errorf("target addr is required")
	}

	return cfg, nil
}

func BuildShellPlan(cfg Config) providerapi.ConnectorPlan {
	args := baseArgs(cfg)
	args = append(args, destination(cfg))
	return providerapi.ConnectorPlan{
		Kind:    "command",
		Program: cfg.SSHPath,
		Args:    args,
		Details: map[string]interface{}{
			"connector": "openssh",
			"operation": "shell",
			"transport": "shell_transport",
		},
	}
}

func BuildExecPlan(cfg Config, op providerapi.ConnectorOperation) providerapi.ConnectorPlan {
	args := baseArgs(cfg)
	if op.Name == "exec_pty" || op.PTY {
		args = append(args, "-tt")
	}
	args = append(args, destination(cfg))
	args = append(args, op.Command...)
	return providerapi.ConnectorPlan{
		Kind:    "command",
		Program: cfg.SSHPath,
		Args:    args,
		Details: map[string]interface{}{
			"connector": "openssh",
			"operation": op.Name,
			"transport": "exec_transport",
		},
	}
}

func BuildSFTPTransportPlan(cfg Config) providerapi.ConnectorPlan {
	args := baseArgs(cfg)
	args = append(args, destination(cfg), "-s", "sftp")
	return providerapi.ConnectorPlan{
		Kind:    "command",
		Program: cfg.SSHPath,
		Args:    args,
		Details: map[string]interface{}{
			"connector": "openssh",
			"operation": "sftp_transport",
			"transport": "sftp_transport",
			"protocol":  "sftp",
		},
	}
}

func BuildLocalForwardPlan(cfg Config, listenHost, listenPort, targetHost, targetPort string) providerapi.ConnectorPlan {
	args := baseArgs(cfg)
	args = append(args, "-N", "-L", fmt.Sprintf("%s:%s:%s", net.JoinHostPort(listenHost, listenPort), targetHost, targetPort))
	args = append(args, destination(cfg))
	return providerapi.ConnectorPlan{
		Kind:    "command",
		Program: cfg.SSHPath,
		Args:    args,
		Details: map[string]interface{}{
			"connector": "openssh",
			"operation": "port_forward_local",
			"transport": "port_forward_transport",
		},
	}
}

func baseArgs(cfg Config) []string {
	args := make([]string, 0, 16)
	if cfg.SSHConfigFile != "" {
		args = append(args, "-F", cfg.SSHConfigFile)
	}
	if cfg.User != "" {
		args = append(args, "-l", cfg.User)
	}
	if cfg.Port != "" {
		args = append(args, "-p", cfg.Port)
	}
	if cfg.IdentityFile != "" {
		args = append(args, "-i", cfg.IdentityFile)
	}
	if cfg.ForwardAgent {
		args = append(args, "-A")
	}
	if cfg.BatchMode {
		args = append(args, "-o", "BatchMode=yes")
	}
	if cfg.UserKnownHostsFile != "" {
		args = append(args, "-o", "UserKnownHostsFile="+cfg.UserKnownHostsFile)
	}
	if cfg.StrictHostKeyChecking != "" {
		args = append(args, "-o", "StrictHostKeyChecking="+cfg.StrictHostKeyChecking)
	}
	for _, option := range cfg.ExtraOptions {
		if strings.TrimSpace(option) == "" {
			continue
		}
		args = append(args, "-o", option)
	}
	return args
}

func destination(cfg Config) string {
	if cfg.User == "" {
		return cfg.Host
	}
	return cfg.User + "@" + cfg.Host
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstStringSlice(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			return append([]string{}, value...)
		}
	}
	return nil
}

func parseBool(raw map[string]interface{}, key string) bool {
	switch strings.ToLower(providerapi.String(raw, key)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
