package main

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/blacknon/lssh/provider/connector/runtimeutil"
	"github.com/blacknon/lssh/provider/mixed/provider-mixed-aws-ec2/eiceconnector"
	ssmconnector "github.com/blacknon/lssh/provider/mixed/provider-mixed-aws-ec2/ssmconnector"
	"github.com/blacknon/lssh/provider/mixed/provider-mixed-aws-ec2/ssmsession"
	"github.com/blacknon/lssh/providerapi"
)

func awsConnectorRunShell(params providerapi.ConnectorRuntimeParams) error {
	switch awsConnectorNameFromRuntime(params) {
	case "aws-ssm":
		return awsSSMRunShell(params)
	default:
		return fmt.Errorf("connector %q does not provide a provider-managed shell runtime", awsConnectorNameFromRuntime(params))
	}
}

func awsConnectorRunExec(params providerapi.ConnectorRuntimeParams) (providerapi.ConnectorExecResult, error) {
	switch awsConnectorNameFromRuntime(params) {
	case "aws-ssm":
		return awsSSMRunExec(params)
	default:
		return providerapi.ConnectorExecResult{}, fmt.Errorf("connector %q does not provide a provider-managed exec runtime", awsConnectorNameFromRuntime(params))
	}
}

func awsSSMRunShell(params providerapi.ConnectorRuntimeParams) error {
	shellConfig, err := ssmconnector.ShellConfigFromPlan(params.Plan)
	if err != nil {
		return err
	}
	if shellConfig.Runtime == "native" {
		if shellConfig.SessionAction != "start" {
			return fmt.Errorf("aws ssm native shell does not support session_action %q yet", shellConfig.SessionAction)
		}
		return runNativeSSMShell(ssmsession.Config{
			InstanceID:             shellConfig.InstanceID,
			Region:                 shellConfig.Region,
			Platform:               shellConfig.Platform,
			Profile:                shellConfig.Profile,
			SharedConfigFiles:      append([]string(nil), shellConfig.SharedConfigFiles...),
			SharedCredentialsFiles: append([]string(nil), shellConfig.SharedCredentialsFiles...),
			DocumentName:           shellConfig.DocumentName,
			StartupCommand:         params.LocalRCCommand,
			StartupMarker:          params.StartupMarker,
		})
	}
	if shellConfig.SessionAction == "detach" {
		sessionID, err := ssmconnector.StartDetachedShell(context.Background(), shellConfig)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(os.Stderr, "Detached Session:%s\n", sessionID)
		return nil
	}
	return ssmconnector.StartShell(context.Background(), shellConfig)
}

func awsSSMRunExec(params providerapi.ConnectorRuntimeParams) (providerapi.ConnectorExecResult, error) {
	if detailString(params.Plan.Details, "shell_runtime") == "native" && detailString(params.Plan.Details, "command_line") != "" {
		cfg, err := ssmconnector.CommandConfigFromPlan(params.Plan)
		if err != nil {
			return providerapi.ConnectorExecResult{}, err
		}
		exitCode, err := ssmsession.RunStreamCommand(context.Background(), ssmsession.Config{
			InstanceID:             cfg.InstanceID,
			Region:                 cfg.Region,
			Platform:               cfg.Platform,
			Profile:                cfg.Profile,
			SharedConfigFiles:      append([]string(nil), cfg.SharedConfigFiles...),
			SharedCredentialsFiles: append([]string(nil), cfg.SharedCredentialsFiles...),
		}, detailString(params.Plan.Details, "command_line"), os.Stdin, os.Stdout, os.Stderr)
		return providerapi.ConnectorExecResult{ExitCode: exitCode}, err
	}

	cmdConfig, err := ssmconnector.CommandConfigFromPlan(params.Plan)
	if err != nil {
		return providerapi.ConnectorExecResult{}, err
	}
	exitCode, err := ssmconnector.RunCommand(context.Background(), cmdConfig, os.Stdout, os.Stderr)
	return providerapi.ConnectorExecResult{ExitCode: exitCode}, err
}

func awsConnectorRunDial(params providerapi.ConnectorRuntimeParams) error {
	var (
		conn net.Conn
		err  error
	)

	switch awsConnectorNameFromRuntime(params) {
	case "aws-eice":
		cfg, cfgErr := eiceconnector.ConfigFromPlan(params.Plan)
		if cfgErr != nil {
			return cfgErr
		}
		conn, err = eiceconnector.DialTarget(context.Background(), cfg)
	case "aws-ssm":
		cfg, cfgErr := ssmconnector.PortForwardDialConfigFromPlan(params.Plan)
		if cfgErr != nil {
			return cfgErr
		}
		conn, err = ssmconnector.DialTarget(context.Background(), cfg)
	default:
		return fmt.Errorf("connector %q does not provide a provider-managed dial runtime", awsConnectorNameFromRuntime(params))
	}
	if err != nil {
		return err
	}
	return runtimeutil.BridgeConn(conn)
}

func awsConnectorNameFromRuntime(params providerapi.ConnectorRuntimeParams) string {
	if connector := detailString(params.Plan.Details, "connector"); connector != "" {
		return connector
	}
	return awsConnectorNameFromPrepare(providerapi.ConnectorPrepareParams{
		Provider: params.Provider,
		Config:   params.Config,
		Target:   params.Target,
	})
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
