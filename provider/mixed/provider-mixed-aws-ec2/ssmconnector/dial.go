package ssmconnector

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/blacknon/lssh/providerapi"
)

type PortForwardDialConfig struct {
	BaseConfig
	DocumentName string
	Runtime      string
	TargetHost   string
	TargetPort   string
}

func PortForwardDialConfigFromPlan(plan providerapi.ConnectorPlan) (PortForwardDialConfig, error) {
	cfg := PortForwardDialConfig{BaseConfig: baseConfigFromPlan(plan)}
	cfg.DocumentName = detailString(plan.Details, "document_name")
	cfg.Runtime = detailString(plan.Details, "port_forward_runtime")
	cfg.TargetHost = detailString(plan.Details, "target_host")
	cfg.TargetPort = detailString(plan.Details, "target_port")
	if cfg.Runtime == "" {
		cfg.Runtime = "plugin"
	}
	if cfg.InstanceID == "" || cfg.Region == "" {
		return PortForwardDialConfig{}, fmt.Errorf("aws ssm dial transport plan is missing instance_id or region")
	}
	if cfg.TargetHost == "" || cfg.TargetPort == "" {
		return PortForwardDialConfig{}, fmt.Errorf("aws ssm dial transport plan is missing target_host or target_port")
	}
	return cfg, nil
}

func DialTarget(ctx context.Context, cfg PortForwardDialConfig) (net.Conn, error) {
	if cfg.Runtime != "plugin" {
		return nil, fmt.Errorf("aws ssm native dial transport is not implemented yet")
	}

	listenAddress, listenPort, err := reserveLoopbackPort()
	if err != nil {
		return nil, err
	}

	commandCfg := PortForwardLocalConfig{
		BaseConfig:   cfg.BaseConfig,
		DocumentName: cfg.DocumentName,
		Runtime:      cfg.Runtime,
		ListenHost:   "127.0.0.1",
		ListenPort:   listenPort,
		TargetHost:   cfg.TargetHost,
		TargetPort:   cfg.TargetPort,
	}

	cmd := BuildStartPortForwardCommand(ctx, commandCfg)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start aws ssm dial transport: %w", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	conn, err := waitForDialTargetReady(ctx, listenAddress, cmd, waitDone, &output)
	if err != nil {
		terminatePortForwardCommand(cmd, waitDone)
		return nil, err
	}

	return &portForwardCommandConn{
		Conn:     conn,
		cmd:      cmd,
		waitDone: waitDone,
	}, nil
}

func reserveLoopbackPort() (string, string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", "", fmt.Errorf("reserve loopback port: %w", err)
	}
	address := listener.Addr().String()
	_ = listener.Close()

	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", "", fmt.Errorf("split reserved loopback port %q: %w", address, err)
	}
	return address, port, nil
}

func waitForDialTargetReady(ctx context.Context, address string, cmd *exec.Cmd, waitDone <-chan error, output *bytes.Buffer) (net.Conn, error) {
	deadlineCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		deadlineCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var lastDialErr error
	for {
		select {
		case err := <-waitDone:
			return nil, fmt.Errorf("aws ssm dial transport exited before connect: %s", formatPortForwardDialError(err, output))
		case <-deadlineCtx.Done():
			if lastDialErr != nil {
				return nil, fmt.Errorf("wait aws ssm dial transport on %s: %w", address, lastDialErr)
			}
			return nil, fmt.Errorf("wait aws ssm dial transport on %s: %w", address, deadlineCtx.Err())
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
			if err == nil {
				return conn, nil
			}
			lastDialErr = err
		}
	}
}

func formatPortForwardDialError(err error, output *bytes.Buffer) string {
	message := "unknown error"
	if err != nil {
		message = err.Error()
	}
	if output == nil {
		return message
	}
	details := strings.TrimSpace(output.String())
	if details == "" {
		return message
	}
	return message + ": " + details
}

func terminatePortForwardCommand(cmd *exec.Cmd, waitDone <-chan error) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	if waitDone != nil {
		<-waitDone
	}
}

type portForwardCommandConn struct {
	net.Conn

	cmd       *exec.Cmd
	waitDone  <-chan error
	closeOnce sync.Once
}

func (c *portForwardCommandConn) Close() error {
	err := c.Conn.Close()
	c.closeOnce.Do(func() {
		terminatePortForwardCommand(c.cmd, c.waitDone)
	})
	return err
}
