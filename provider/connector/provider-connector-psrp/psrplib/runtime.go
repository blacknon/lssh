package psrplib

import (
	"fmt"
	"strings"

	"github.com/blacknon/lssh/providerapi"
)

const (
	RuntimeCommand = "command"
	RuntimeLibrary = "library"
)

type RuntimeConfig struct {
	Shell string
	Exec  string
}

func ResolveRuntime(providerConfig, targetConfig map[string]interface{}) RuntimeConfig {
	shell := firstNonEmpty(
		stringValue(targetConfig, "shell_runtime", ""),
		stringValue(targetConfig, "runtime", ""),
		stringValue(providerConfig, "shell_runtime", ""),
		stringValue(providerConfig, "runtime", ""),
		RuntimeCommand,
	)
	exec := firstNonEmpty(
		stringValue(targetConfig, "exec_runtime", ""),
		stringValue(targetConfig, "runtime", ""),
		stringValue(providerConfig, "exec_runtime", ""),
		stringValue(providerConfig, "runtime", ""),
		RuntimeCommand,
	)

	return RuntimeConfig{
		Shell: normalizeRuntime(shell),
		Exec:  normalizeRuntime(exec),
	}
}

func normalizeRuntime(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case RuntimeLibrary:
		return RuntimeLibrary
	default:
		return RuntimeCommand
	}
}

func RuntimeSupported(runtime string) bool {
	switch normalizeRuntime(runtime) {
	case RuntimeCommand:
		return true
	case RuntimeLibrary:
		return true
	default:
		return false
	}
}

func RuntimeUnsupportedReason(runtime string) string {
	switch normalizeRuntime(runtime) {
	case RuntimeLibrary:
		return ""
	default:
		return ""
	}
}

func BuildPlan(cfg Config, runtime string, op providerapi.ConnectorOperation) (providerapi.ConnectorPlan, error) {
	switch normalizeRuntime(runtime) {
	case RuntimeCommand:
		switch op.Name {
		case "shell":
			return BuildShellPlan(cfg), nil
		case "exec":
			return BuildExecPlan(cfg, op), nil
		default:
			return providerapi.ConnectorPlan{}, fmt.Errorf("operation %q is not supported by the psrp connector", op.Name)
		}
	case RuntimeLibrary:
		switch op.Name {
		case "shell":
			return BuildLibraryShellPlan(cfg)
		case "exec":
			return BuildLibraryExecPlan(cfg, op)
		default:
			return providerapi.ConnectorPlan{}, fmt.Errorf("operation %q is not supported by the psrp connector", op.Name)
		}
	default:
		return providerapi.ConnectorPlan{}, fmt.Errorf("unknown psrp runtime %q", runtime)
	}
}
