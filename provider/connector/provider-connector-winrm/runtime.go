package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/blacknon/lssh/provider/connector/provider-connector-winrm/winrmlib"
	"github.com/blacknon/lssh/provider/connector/runtimeutil"
	"github.com/blacknon/lssh/providerapi"
)

func winrmRunShell(params providerapi.ConnectorRuntimeParams) error {
	cfg, err := winrmlib.ConfigFromPlanDetails(params.Plan.Details)
	if err != nil {
		return err
	}
	connect, err := winrmlib.CreateConnect(cfg)
	if err != nil {
		return err
	}
	terminal, err := connect.OpenShell()
	if err != nil {
		return err
	}
	defer terminal.Close()
	return runtimeutil.StreamInteractiveSession(terminal.Stdin(), terminal.Stdout(), terminal.Stderr(), terminal.Wait)
}

func winrmRunExec(params providerapi.ConnectorRuntimeParams) (providerapi.ConnectorExecResult, error) {
	cfg, err := winrmlib.ConfigFromPlanDetails(params.Plan.Details)
	if err != nil {
		return providerapi.ConnectorExecResult{}, err
	}
	connect, err := winrmlib.CreateConnect(cfg)
	if err != nil {
		return providerapi.ConnectorExecResult{}, err
	}
	commandLine := detailString(params.Plan.Details, "command_line")
	if commandLine == "" {
		return providerapi.ConnectorExecResult{}, fmt.Errorf("winrm command plan is missing command_line")
	}
	command, err := connect.ExecuteCommand(commandLine)
	if err != nil {
		return providerapi.ConnectorExecResult{}, err
	}
	defer command.Close()

	var wg sync.WaitGroup
	if stdout := command.Stdout(); stdout != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runtimeutil.CopyStream(os.Stdout, stdout)
		}()
	}
	if stderr := command.Stderr(); stderr != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runtimeutil.CopyStream(os.Stderr, stderr)
		}()
	}
	waitErr := command.Wait()
	wg.Wait()
	return providerapi.ConnectorExecResult{ExitCode: command.ExitCode()}, waitErr
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
