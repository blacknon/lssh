package providerapi

import "encoding/json"

const Version = "v1"

const (
	MethodPluginDescribe    = "plugin.describe"
	MethodInventoryList     = "inventory.list"
	MethodSecretGet         = "secret.get"
	MethodHealthCheck       = "health.check"
	MethodConnectorDescribe = "connector.describe"
	MethodConnectorPrepare  = "connector.prepare"
	// MethodTransportPrep is kept as a legacy alias while the connector API
	// moves from transport.* naming to connector.* naming.
	MethodTransportPrep = "transport.prepare"
)

type Request struct {
	Version string      `json:"version"`
	ID      string      `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type Response struct {
	Version  string          `json:"version"`
	ID       string          `json:"id,omitempty"`
	Result   json.RawMessage `json:"result,omitempty"`
	Error    *Error          `json:"error,omitempty"`
	Warnings []Warning       `json:"warnings,omitempty"`
}

type Error struct {
	Code      string                 `json:"code,omitempty"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Retryable bool                   `json:"retryable,omitempty"`
}

type Warning struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type PluginDescribeResult struct {
	Name            string   `json:"name"`
	Capabilities    []string `json:"capabilities,omitempty"`
	ConnectorNames  []string `json:"connector_names,omitempty"`
	Methods         []string `json:"methods,omitempty"`
	ProtocolVersion string   `json:"protocol_version,omitempty"`
	PluginVersion   string   `json:"plugin_version,omitempty"`
}

type InventoryListParams struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

type InventoryServer struct {
	Name   string                 `json:"name"`
	Config map[string]interface{} `json:"config,omitempty"`
	Meta   map[string]string      `json:"meta,omitempty"`
}

type InventoryListResult struct {
	Servers []InventoryServer `json:"servers,omitempty"`
}

type SecretGetParams struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config,omitempty"`
	Ref      string                 `json:"ref"`
	Server   string                 `json:"server,omitempty"`
	Field    string                 `json:"field,omitempty"`
}

type SecretGetResult struct {
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}

type HealthCheckParams struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

type HealthCheckResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type ConnectorTarget struct {
	Name   string                 `json:"name,omitempty"`
	Config map[string]interface{} `json:"config,omitempty"`
	Meta   map[string]string      `json:"meta,omitempty"`
}

type ConnectorDescribeParams struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config,omitempty"`
	Target   ConnectorTarget        `json:"target"`
}

type ConnectorCapability struct {
	Supported   bool                   `json:"supported"`
	Reason      string                 `json:"reason,omitempty"`
	Constraints map[string]interface{} `json:"constraints,omitempty"`
	Requires    []string               `json:"requires,omitempty"`
	Preferred   bool                   `json:"preferred,omitempty"`
}

type ConnectorDescribeResult struct {
	Capabilities map[string]ConnectorCapability `json:"capabilities,omitempty"`
}

type ConnectorOperation struct {
	Name    string                 `json:"name"`
	Command []string               `json:"command,omitempty"`
	Env     map[string]string      `json:"env,omitempty"`
	Cwd     string                 `json:"cwd,omitempty"`
	PTY     bool                   `json:"pty,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type ConnectorPrepareParams struct {
	Provider  string                 `json:"provider"`
	Config    map[string]interface{} `json:"config,omitempty"`
	Target    ConnectorTarget        `json:"target"`
	Operation ConnectorOperation     `json:"operation"`
}

type ConnectorPlan struct {
	Kind    string                 `json:"kind"`
	Program string                 `json:"program,omitempty"`
	Args    []string               `json:"args,omitempty"`
	Env     map[string]string      `json:"env,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

type ConnectorPrepareResult struct {
	Supported bool          `json:"supported"`
	Plan      ConnectorPlan `json:"plan,omitempty"`
}
