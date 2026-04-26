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
	Supported           bool
	ConnectorName       string
	Reason              string
	PlanKind            string
	Command             *ConnectorCommandPlan
	ManagedSSH          *ConnectorManagedSSHPlan
	ProviderManagedPlan *providerapi.ConnectorPlan
	SupportsLocalRC     bool
	SharedShellForward  bool
}

type ConnectorCommandPlan struct {
	Program string
	Args    []string
	Env     map[string]string
}

type ConnectorManagedSSHPlan struct {
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

	return preparedConnectorFromProvider(prepared, connectorName, operation), nil
}

func preparedConnectorFromProvider(result providerapi.ConnectorPrepareResult, fallback string, operation ConnectorOperation) PreparedConnector {
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
		planCopy := result.Plan
		prepared.ProviderManagedPlan = &planCopy
		prepared.SupportsLocalRC = providerManagedSupportsLocalRC(result.Plan, operation)
		prepared.SharedShellForward = providerManagedSharesShellForward(result.Plan, operation)
		if managed := managedSSHPlanFromProvider(result.Plan, operation); managed != nil {
			prepared.ManagedSSH = managed
		}
	}

	return prepared
}

func managedSSHPlanFromProvider(plan providerapi.ConnectorPlan, operation ConnectorOperation) *ConnectorManagedSSHPlan {
	if plan.Kind != "provider-managed" {
		return nil
	}

	transport := detailString(plan.Details, "transport")
	sshRuntime := detailString(plan.Details, "ssh_runtime")
	shellRuntime := detailString(plan.Details, "shell_runtime")
	portForwardRuntime := detailString(plan.Details, "port_forward_runtime")
	sessionAction := detailString(plan.Details, "session_action")

	switch {
	case transport == "ssh_transport" && sshRuntime == "sdk" && operation.Name != "tcp_dial_transport":
		return &ConnectorManagedSSHPlan{}
	default:
		_ = shellRuntime
		_ = portForwardRuntime
		_ = sessionAction
		return nil
	}
}

func providerManagedSupportsLocalRC(plan providerapi.ConnectorPlan, operation ConnectorOperation) bool {
	return plan.Kind == "provider-managed" &&
		operation.Name == "shell" &&
		detailString(plan.Details, "shell_runtime") == "native" &&
		detailString(plan.Details, "session_action") == "start"
}

func providerManagedSharesShellForward(plan providerapi.ConnectorPlan, operation ConnectorOperation) bool {
	if plan.Kind != "provider-managed" {
		return false
	}
	if operation.Name == "shell" &&
		detailString(plan.Details, "shell_runtime") == "native" &&
		detailString(plan.Details, "session_action") == "start" {
		return true
	}
	return false
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
