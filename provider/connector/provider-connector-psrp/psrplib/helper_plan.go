package psrplib

import (
	"fmt"
	"os"

	"github.com/blacknon/lssh/providerapi"
)

func BuildLibraryShellPlan(cfg Config) (providerapi.ConnectorPlan, error) {
	program, err := os.Executable()
	if err != nil {
		return providerapi.ConnectorPlan{}, fmt.Errorf("resolve psrp helper executable: %w", err)
	}

	return providerapi.ConnectorPlan{
		Kind:    "command",
		Program: program,
		Args:    []string{helperCommand, helperShell},
		Env:     buildBaseEnv(cfg, ""),
		Details: map[string]interface{}{
			"connector": "psrp",
			"operation": "shell",
			"runtime":   RuntimeLibrary,
			"transport": "shell_transport",
		},
	}, nil
}

func BuildLibraryExecPlan(cfg Config, op providerapi.ConnectorOperation) (providerapi.ConnectorPlan, error) {
	program, err := os.Executable()
	if err != nil {
		return providerapi.ConnectorPlan{}, fmt.Errorf("resolve psrp helper executable: %w", err)
	}

	commandLine := ""
	if len(op.Command) > 0 {
		commandLine = op.Command[0]
		for _, part := range op.Command[1:] {
			commandLine += " " + part
		}
	}

	return providerapi.ConnectorPlan{
		Kind:    "command",
		Program: program,
		Args:    []string{helperCommand, helperExec},
		Env:     buildBaseEnv(cfg, commandLine),
		Details: map[string]interface{}{
			"connector":    "psrp",
			"operation":    op.Name,
			"runtime":      RuntimeLibrary,
			"transport":    "exec_transport",
			"command":      append([]string(nil), op.Command...),
			"command_line": commandLine,
		},
	}, nil
}
