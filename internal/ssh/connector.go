package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/providerapi"
	ssmconnector "github.com/blacknon/lssh/provider/mixed/provider-mixed-aws-ec2/ssmconnector"
)

func (r *Run) UsesConnector(server string) bool {
	return r.Conf.ServerUsesConnector(server)
}

func (r *Run) usesConnector(server string) bool {
	return r.UsesConnector(server)
}

func (r *Run) RunConnectorShell(server string) error {
	config := r.Conf.Server[server]
	if connectorName(config) != "aws-ssm" {
		return fmt.Errorf("server %q connector %q is not supported yet", server, connectorName(config))
	}
	if connectorHasForwarding(config) {
		return fmt.Errorf("server %q uses connector %q; ssh port forwarding is not supported", server, connectorName(config))
	}
	if r.IsBashrc || r.IsNotBashrc || config.LocalRcUse == "yes" {
		return fmt.Errorf("server %q uses connector %q; localrc is not supported", server, connectorName(config))
	}

	r.PrintSelectServer()

	plan, err := r.Conf.PrepareConnector(server, providerapi.ConnectorOperation{Name: "shell"})
	if err != nil {
		return err
	}
	if !plan.Supported {
		return fmt.Errorf("server %q connector %q: %v", server, connectorName(config), plan.Plan.Details["reason"])
	}

	shellConfig, err := ssmconnector.ShellConfigFromPlan(plan.Plan)
	if err != nil {
		return err
	}
	return ssmconnector.StartShell(context.Background(), shellConfig)
}

func (r *Run) runConnectorShell(server string) error {
	return r.RunConnectorShell(server)
}

func (r *Run) RunConnectorCommand(server string, command []string, stdout, stderr io.Writer) (int, error) {
	config := r.Conf.Server[server]
	if connectorName(config) != "aws-ssm" {
		return 0, fmt.Errorf("server %q connector %q is not supported yet", server, connectorName(config))
	}

	plan, err := r.Conf.PrepareConnector(server, providerapi.ConnectorOperation{
		Name:    "exec",
		Command: append([]string(nil), command...),
	})
	if err != nil {
		return 0, err
	}
	if !plan.Supported {
		return 0, fmt.Errorf("server %q connector %q: %v", server, connectorName(config), plan.Plan.Details["reason"])
	}

	cmdConfig, err := ssmconnector.CommandConfigFromPlan(plan.Plan)
	if err != nil {
		return 0, err
	}
	return ssmconnector.RunCommand(context.Background(), cmdConfig, stdout, stderr)
}

func (r *Run) runConnectorCommand(server string, stdout, stderr io.Writer) (int, error) {
	return r.RunConnectorCommand(server, r.ExecCmd, stdout, stderr)
}

func connectorName(config conf.ServerConfig) string {
	if config.ProviderPlugin == "provider-mixed-aws-ec2" {
		return "aws-ssm"
	}
	return ""
}

func connectorHasForwarding(config conf.ServerConfig) bool {
	return len(config.Forwards) > 0 ||
		config.DynamicPortForward != "" ||
		config.ReverseDynamicPortForward != "" ||
		config.HTTPDynamicPortForward != "" ||
		config.HTTPReverseDynamicPortForward != "" ||
		config.NFSDynamicForwardPort != "" ||
		config.NFSReverseDynamicForwardPort != "" ||
		config.SMBDynamicForwardPort != "" ||
		config.SMBReverseDynamicForwardPort != ""
}

func connectorErrorString(server string, err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return fmt.Sprintf("%s :: connector execution failed", server)
	}
	return fmt.Sprintf("%s :: %s", server, message)
}

func connectorOutputWriters(r *Run, server string, single bool) (io.Writer, io.Writer) {
	if single && r.IsTerm {
		return os.Stdout, os.Stderr
	}
	o := r.createCommandOutput(server)
	return o.NewWriter(), o.NewWriter()
}
