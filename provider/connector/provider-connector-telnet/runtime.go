package main

import (
	"net"
	"os"
	"time"

	"github.com/blacknon/lssh/provider/connector/provider-connector-telnet/telnetlib"
	"github.com/blacknon/lssh/providerutil/runtimeutil"
	"github.com/blacknon/lssh/providerapi"
)

func telnetRunShell(params providerapi.ConnectorRuntimeParams) error {
	cfg, err := telnetlib.ConfigFromPlanDetails(params.Plan.Details)
	if err != nil {
		return err
	}
	terminal, err := openTelnetShell(cfg)
	if err != nil {
		return err
	}
	return runtimeutil.StreamInteractiveSession(terminal.Stdin(), terminal.Stdout(), terminal.Stderr(), func() error {
		waitErr := terminal.Wait()
		_ = terminal.Close()
		return waitErr
	})
}

func openTelnetShell(cfg telnetlib.TargetConfig) (*telnetlib.Terminal, error) {
	bridgeAddr := providerapi.RuntimeBridgeAddrEnvVar
	if value := os.Getenv(bridgeAddr); value != "" {
		timeout := time.Duration(cfg.DialTimeoutSec) * time.Second
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		conn, err := net.DialTimeout("tcp", value, timeout)
		if err != nil {
			return nil, err
		}
		return telnetlib.OpenShellWithConn(cfg, conn, "xterm-256color", 80, 24)
	}

	return telnetlib.OpenShell(cfg, "xterm-256color", 80, 24)
}
