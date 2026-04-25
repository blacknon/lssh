package main

import (
	"context"

	"github.com/blacknon/lssh/provider/connector/runtimeutil"
	"github.com/blacknon/lssh/provider/mixed/provider-mixed-azure-compute/bastionconnector"
	"github.com/blacknon/lssh/providerapi"
)

func azureConnectorRunDial(params providerapi.ConnectorRuntimeParams) error {
	cfg, err := bastionconnector.ConfigFromPlan(params.Plan)
	if err != nil {
		return err
	}
	conn, err := bastionconnector.DialTarget(context.Background(), cfg)
	if err != nil {
		return err
	}
	return runtimeutil.BridgeConn(conn)
}
