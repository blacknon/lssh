package conf

import (
	"fmt"
	"strings"

	"github.com/blacknon/lssh/providerapi"
)

type ConnectorOperation struct {
	Name    string
	Command []string
	Env     map[string]string
	Cwd     string
	PTY     bool
	Options map[string]interface{}
}

type PreparedConnector struct {
	Supported     bool
	ConnectorName string
	Reason        string
	PlanKind      string
	Command       *ConnectorCommandPlan
	ManagedSSH    *ConnectorManagedSSHPlan
}

type ConnectorCommandPlan struct {
	Program string
	Args    []string
	Env     map[string]string
}

type ConnectorManagedSSHPlan struct {
	ShareForwardingWithShell bool
	SupportsLocalRC          bool
}

func (c *Config) PrepareConnectorRuntime(server string, operation ConnectorOperation) (PreparedConnector, error) {
	serverConfig, ok := c.Server[server]
	if !ok {
		return PreparedConnector{}, fmt.Errorf("server %q is not configured", server)
	}

	connectorName := strings.TrimSpace(c.serverConnectorName(serverConfig))
	if connectorName == "" || connectorName == "ssh" {
		return PreparedConnector{}, fmt.Errorf("server %q does not use an external connector", server)
	}

	prepared, err := c.PrepareConnector(server, providerapi.ConnectorOperation{
		Name:    operation.Name,
		Command: append([]string(nil), operation.Command...),
		Env:     cloneStringMap(operation.Env),
		Cwd:     operation.Cwd,
		PTY:     operation.PTY,
		Options: cloneInterfaceMap(operation.Options),
	})
	if err != nil {
		return PreparedConnector{}, err
	}

	return preparedConnectorFromProvider(prepared, connectorName), nil
}

func (c *Config) ConnectorSupportsSessionControl(server string) bool {
	return strings.TrimSpace(c.ServerConnectorName(server)) == "aws-ssm"
}

func (c *Config) ConnectorSupportsLSPipe(server string) bool {
	return strings.TrimSpace(c.ServerConnectorName(server)) == "aws-ssm"
}

func preparedConnectorFromProvider(result providerapi.ConnectorPrepareResult, fallback string) PreparedConnector {
	connectorName := connectorNameFromPlan(result.Plan, fallback)
	prepared := PreparedConnector{
		Supported:     result.Supported,
		ConnectorName: connectorName,
		Reason:        detailString(result.Plan.Details, "reason"),
		PlanKind:      result.Plan.Kind,
	}

	switch result.Plan.Kind {
	case "command":
		prepared.Command = &ConnectorCommandPlan{
			Program: result.Plan.Program,
			Args:    append([]string(nil), result.Plan.Args...),
			Env:     cloneStringMap(result.Plan.Env),
		}
	case "provider-managed":
		if managed := managedSSHPlanFromProvider(result.Plan, connectorName); managed != nil {
			prepared.ManagedSSH = managed
		}
	}

	return prepared
}

func managedSSHPlanFromProvider(plan providerapi.ConnectorPlan, connectorName string) *ConnectorManagedSSHPlan {
	if plan.Kind != "provider-managed" {
		return nil
	}
	if detailString(plan.Details, "ssh_runtime") != "sdk" {
		return nil
	}

	transport := detailString(plan.Details, "transport")
	shellRuntime := detailString(plan.Details, "shell_runtime")
	if transport != "ssh_transport" && !(connectorName == "aws-ssm" && shellRuntime == "native") {
		return nil
	}

	return &ConnectorManagedSSHPlan{
		ShareForwardingWithShell: connectorName == "aws-ssm" && shellRuntime == "native",
		SupportsLocalRC:          true,
	}
}

func connectorNameFromPlan(plan providerapi.ConnectorPlan, fallback string) string {
	if value := detailString(plan.Details, "connector"); value != "" {
		return value
	}
	return fallback
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

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}

	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func cloneInterfaceMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return nil
	}

	dst := make(map[string]interface{}, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
