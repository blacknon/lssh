package conf

import (
	"fmt"
	"sort"
	"strings"

	"github.com/blacknon/lssh/internal/providerapi"
)

func (c *Config) ServerConnectorName(server string) string {
	serverConfig, ok := c.Server[server]
	if !ok {
		return ""
	}
	return c.serverConnectorName(serverConfig)
}

func (c *Config) ServerUsesConnector(server string) bool {
	connectorName := strings.TrimSpace(c.ServerConnectorName(server))
	return connectorName != "" && connectorName != "ssh"
}

func (c *Config) ServerUsesBuiltInSSH(server string) bool {
	connectorName := strings.TrimSpace(c.ServerConnectorName(server))
	return connectorName == "" || connectorName == "ssh"
}

func (c *Config) PrepareConnector(server string, operation providerapi.ConnectorOperation) (providerapi.ConnectorPrepareResult, error) {
	serverConfig, ok := c.Server[server]
	if !ok {
		return providerapi.ConnectorPrepareResult{}, fmt.Errorf("server %q is not configured", server)
	}

	connectorName := strings.TrimSpace(c.serverConnectorName(serverConfig))
	if connectorName == "" || connectorName == "ssh" {
		return providerapi.ConnectorPrepareResult{}, fmt.Errorf("server %q does not use an external connector", server)
	}

	providerName, raw, err := c.resolveConnectorProvider(serverConfig, connectorName)
	if err != nil {
		return providerapi.ConnectorPrepareResult{}, err
	}

	var result providerapi.ConnectorPrepareResult
	if err := c.callProvider(providerName, providerapi.MethodConnectorPrepare, providerapi.ConnectorPrepareParams{
		Provider: providerName,
		Config:   raw,
		Target: providerapi.ConnectorTarget{
			Name:   server,
			Config: serverConfigToTOMLMap(serverConfig),
			Meta:   cloneProviderMeta(serverConfig.ProviderMeta),
		},
		Operation: operation,
	}, &result); err != nil {
		return providerapi.ConnectorPrepareResult{}, err
	}

	return result, nil
}

func (c *Config) DescribeConnector(server string) (providerapi.ConnectorDescribeResult, error) {
	serverConfig, ok := c.Server[server]
	if !ok {
		return providerapi.ConnectorDescribeResult{}, fmt.Errorf("server %q is not configured", server)
	}

	connectorName := strings.TrimSpace(c.serverConnectorName(serverConfig))
	if connectorName == "" || connectorName == "ssh" {
		return providerapi.ConnectorDescribeResult{}, fmt.Errorf("server %q does not use an external connector", server)
	}

	providerName, raw, err := c.resolveConnectorProvider(serverConfig, connectorName)
	if err != nil {
		return providerapi.ConnectorDescribeResult{}, err
	}

	var result providerapi.ConnectorDescribeResult
	if err := c.callProvider(providerName, providerapi.MethodConnectorDescribe, providerapi.ConnectorDescribeParams{
		Provider: providerName,
		Config:   raw,
		Target: providerapi.ConnectorTarget{
			Name:   server,
			Config: serverConfigToTOMLMap(serverConfig),
			Meta:   cloneProviderMeta(serverConfig.ProviderMeta),
		},
	}, &result); err != nil {
		return providerapi.ConnectorDescribeResult{}, err
	}

	return result, nil
}

func (c *Config) ServerSupportsOperation(server string, operation string) (bool, error) {
	if !c.ServerUsesConnector(server) {
		return true, nil
	}

	describe, err := c.DescribeConnector(server)
	if err != nil {
		return false, err
	}

	capability, ok := describe.Capabilities[operation]
	if !ok {
		return false, nil
	}

	return capability.Supported, nil
}

func (c *Config) FilterServersByOperation(servers []string, operation string) ([]string, error) {
	filtered := make([]string, 0, len(servers))
	for _, server := range servers {
		supported, err := c.ServerSupportsOperation(server, operation)
		if err != nil {
			return nil, err
		}
		if supported {
			filtered = append(filtered, server)
		}
	}

	return filtered, nil
}

func (c *Config) serverConnectorName(serverConfig ServerConfig) string {
	if connectorName := strings.TrimSpace(serverConfig.ConnectorName); connectorName != "" {
		return connectorName
	}

	if serverConfig.ProviderName == "" {
		return ""
	}

	raw, ok := c.Provider[serverConfig.ProviderName]
	if !ok || !providerEnabled(raw) || !providerHasCapability(raw, "connector") {
		return ""
	}
	if defaultConnector := strings.TrimSpace(providerString(raw, "default_connector_name")); defaultConnector != "" {
		return defaultConnector
	}

	describe, err := c.describeProvider(serverConfig.ProviderName)
	if err != nil || len(describe.ConnectorNames) != 1 {
		return ""
	}

	return strings.TrimSpace(describe.ConnectorNames[0])
}

func (c *Config) describeProvider(providerName string) (providerapi.PluginDescribeResult, error) {
	if _, ok := c.Provider[providerName]; !ok {
		return providerapi.PluginDescribeResult{}, fmt.Errorf("provider %q is not configured", providerName)
	}

	var result providerapi.PluginDescribeResult
	if err := c.callProvider(providerName, providerapi.MethodPluginDescribe, nil, &result); err != nil {
		return providerapi.PluginDescribeResult{}, err
	}
	return result, nil
}

func (c *Config) resolveConnectorProvider(serverConfig ServerConfig, connectorName string) (string, map[string]interface{}, error) {
	if serverConfig.ProviderName != "" {
		if raw, ok := c.Provider[serverConfig.ProviderName]; ok && providerEnabled(raw) && providerHasCapability(raw, "connector") {
			describe, err := c.describeProvider(serverConfig.ProviderName)
			if err == nil && stringSliceContains(describe.ConnectorNames, connectorName) {
				return serverConfig.ProviderName, raw, nil
			}
		}
	}

	return c.resolveConfiguredConnectorProvider(connectorName)
}

func (c *Config) resolveConfiguredConnectorProvider(connectorName string) (string, map[string]interface{}, error) {
	names := make([]string, 0, len(c.Provider))
	for name := range c.Provider {
		names = append(names, name)
	}
	sort.Strings(names)

	var matched string
	var matchedRaw map[string]interface{}

	for _, name := range names {
		raw := c.Provider[name]
		if !providerEnabled(raw) || !providerHasCapability(raw, "connector") {
			continue
		}

		describe, err := c.describeProvider(name)
		if err != nil {
			return "", nil, err
		}
		if !stringSliceContains(describe.ConnectorNames, connectorName) {
			continue
		}
		if matched != "" {
			return "", nil, fmt.Errorf("connector %q is provided by multiple providers: %q and %q", connectorName, matched, name)
		}

		matched = name
		matchedRaw = raw
	}

	if matched == "" {
		return "", nil, fmt.Errorf("connector %q is not configured", connectorName)
	}

	return matched, matchedRaw, nil
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == needle {
			return true
		}
	}
	return false
}
