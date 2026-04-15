package providerapi

import "encoding/json"

const Version = "v1"

const (
	MethodPluginDescribe = "plugin.describe"
	MethodInventoryList  = "inventory.list"
	MethodSecretGet      = "secret.get"
	MethodHealthCheck    = "health.check"
	MethodTransportPrep  = "transport.prepare"
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
