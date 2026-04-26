package main

import (
	"context"

	"github.com/blacknon/lssh/providerutil/runtimeutil"
	"github.com/blacknon/lssh/provider/mixed/provider-mixed-gcp-compute/iapconnector"
	"github.com/blacknon/lssh/providerapi"
)

func gcpConnectorRunDial(params providerapi.ConnectorRuntimeParams) error {
	cfg, err := iapconnector.ConfigFromPlan(params.Plan)
	if err != nil {
		return err
	}
	conn, err := iapconnector.DialTarget(context.Background(), cfg)
	if err != nil {
		return err
	}
	return runtimeutil.BridgeConn(conn)
}
