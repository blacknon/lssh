package conf

import (
	"fmt"

	"github.com/blacknon/lssh/internal/providerapi"
)

func (c *Config) ServerUsesConnector(server string) bool {
	serverConfig, ok := c.Server[server]
	if !ok {
		return false
	}
	raw, ok := c.Provider[serverConfig.ProviderName]
	if !ok {
		return false
	}
	return providerEnabled(raw) && providerHasCapability(raw, "connector")
}

func (c *Config) PrepareConnector(server string, operation providerapi.ConnectorOperation) (providerapi.ConnectorPrepareResult, error) {
	serverConfig, ok := c.Server[server]
	if !ok {
		return providerapi.ConnectorPrepareResult{}, fmt.Errorf("server %q is not configured", server)
	}
	raw, ok := c.Provider[serverConfig.ProviderName]
	if !ok {
		return providerapi.ConnectorPrepareResult{}, fmt.Errorf("provider %q is not configured", serverConfig.ProviderName)
	}
	if !providerEnabled(raw) {
		return providerapi.ConnectorPrepareResult{}, fmt.Errorf("provider %q is disabled", serverConfig.ProviderName)
	}
	if !providerHasCapability(raw, "connector") {
		return providerapi.ConnectorPrepareResult{}, fmt.Errorf("provider %q does not support connector capability", serverConfig.ProviderName)
	}

	var result providerapi.ConnectorPrepareResult
	if err := c.callProvider(serverConfig.ProviderName, providerapi.MethodConnectorPrepare, providerapi.ConnectorPrepareParams{
		Provider: serverConfig.ProviderName,
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
