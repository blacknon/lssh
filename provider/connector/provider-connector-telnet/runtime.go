package main

import (
	"github.com/blacknon/lssh/provider/connector/provider-connector-telnet/telnetlib"
	"github.com/blacknon/lssh/provider/connector/runtimeutil"
	"github.com/blacknon/lssh/providerapi"
)

func telnetRunShell(params providerapi.ConnectorRuntimeParams) error {
	cfg, err := telnetlib.ConfigFromPlanDetails(params.Plan.Details)
	if err != nil {
		return err
	}
	terminal, err := telnetlib.OpenShell(cfg, "xterm-256color", 80, 24)
	if err != nil {
		return err
	}
	defer terminal.Close()
	return runtimeutil.StreamInteractiveSession(terminal.Stdin(), terminal.Stdout(), terminal.Stderr(), terminal.Wait)
}
