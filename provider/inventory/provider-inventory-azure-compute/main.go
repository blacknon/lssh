package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/blacknon/lssh/internal/providerapi"
	"github.com/blacknon/lssh/internal/providerbuiltin"
)

const (
	azureComputeAPIVersion = "2025-04-01"
	azureNetworkAPIVersion = "2024-07-01"
)

func main() {
	req, err := providerbuiltin.ReadRequest()
	if err != nil {
		_ = providerbuiltin.WriteError(err.Error())
		os.Exit(1)
	}

	switch req.Method {
	case providerapi.MethodPluginDescribe:
		_ = providerbuiltin.WriteResponse(req, providerapi.PluginDescribeResult{
			Name:            "provider-inventory-azure-compute",
			Capabilities:    []string{"inventory"},
			Methods:         []string{providerapi.MethodPluginDescribe, providerapi.MethodHealthCheck, providerapi.MethodInventoryList},
			ProtocolVersion: providerapi.Version,
		}, nil)
	case providerapi.MethodInventoryList:
		var params providerapi.InventoryListParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		servers, err := listAzure(params.Config)
		if err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "inventory_failed", err.Error())
			os.Exit(1)
		}
		_ = providerbuiltin.WriteResponse(req, providerapi.InventoryListResult{Servers: servers}, nil)
	case providerapi.MethodHealthCheck:
		var params providerapi.HealthCheckParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := azureHealthCheck(params.Config)
		if err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "health_check_failed", err.Error())
			os.Exit(1)
		}
		_ = providerbuiltin.WriteResponse(req, result, nil)
	default:
		_ = providerbuiltin.WriteErrorResponse(req, "unsupported_method", fmt.Sprintf("unsupported method %q", req.Method))
		os.Exit(1)
	}
}

func decodeParams(raw interface{}, out interface{}) error {
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

type azureClient struct {
	baseURL    string
	httpClient *http.Client
	token      string

	nicCache      map[string]azureNIC
	publicIPCache map[string]string
}

type azureVMListResponse struct {
	Value    []azureVM `json:"value"`
	NextLink string    `json:"nextLink"`
}

type azureVM struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Location   string            `json:"location"`
	Tags       map[string]string `json:"tags"`
	Properties struct {
		ProvisioningState string `json:"provisioningState"`
		StorageProfile    struct {
			OSDisk struct {
				OSType string `json:"osType"`
			} `json:"osDisk"`
		} `json:"storageProfile"`
		NetworkProfile struct {
			NetworkInterfaces []struct {
				ID string `json:"id"`
			} `json:"networkInterfaces"`
		} `json:"networkProfile"`
		InstanceView struct {
			Statuses []azureStatus `json:"statuses"`
		} `json:"instanceView"`
	} `json:"properties"`
}

type azureStatus struct {
	Code          string `json:"code"`
	DisplayStatus string `json:"displayStatus"`
}

type azureNIC struct {
	Properties struct {
		IPConfigurations []struct {
			Properties struct {
				PrivateIPAddress string `json:"privateIPAddress"`
				PublicIPAddress  struct {
					ID string `json:"id"`
				} `json:"publicIPAddress"`
			} `json:"properties"`
		} `json:"ipConfigurations"`
	} `json:"properties"`
}

type azurePublicIP struct {
	Properties struct {
		IPAddress string `json:"ipAddress"`
	} `json:"properties"`
}

func listAzure(config map[string]interface{}) ([]providerapi.InventoryServer, error) {
	client, err := newAzureClient(context.Background(), config)
	if err != nil {
		return nil, err
	}

	vms, err := client.listVMs(config, false)
	if err != nil {
		return nil, err
	}
	statuses, err := client.listVMs(config, true)
	if err != nil {
		return nil, err
	}
	powerStateByID := azurePowerStateMap(statuses)

	nameTemplate := providerbuiltin.String(config, "server_name_template")
	if nameTemplate == "" {
		nameTemplate = "azure:${name}"
	}
	noteTemplate := providerbuiltin.String(config, "note_template")
	includeTags := providerbuiltin.StringSlice(config, "include_tags")
	filter := azureInventoryFilterFromConfig(config)

	out := make([]providerapi.InventoryServer, 0, len(vms))
	for _, vm := range vms {
		powerState := powerStateByID[strings.ToLower(vm.ID)]
		if powerState == "" {
			powerState = azurePowerState(vm.Properties.InstanceView.Statuses)
		}
		if !filter.matches(powerState) {
			continue
		}

		privateIP, publicIP, err := client.vmIPs(vm)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to resolve VM network for %s: %v\n", vm.Name, err)
		}
		resourceGroup := azureResourceGroup(vm.ID)
		osType := strings.ToLower(vm.Properties.StorageProfile.OSDisk.OSType)
		meta := map[string]string{
			"provider":           "azure",
			"plugin":             "provider-inventory-azure-compute",
			"id":                 vm.ID,
			"name":               vm.Name,
			"subscription_id":    providerbuiltin.String(config, "subscription_id"),
			"resource_group":     resourceGroup,
			"location":           vm.Location,
			"private_ip":         privateIP,
			"public_ip":          publicIP,
			"power_state":        powerState,
			"provisioning_state": vm.Properties.ProvisioningState,
			"os_type":            osType,
		}
		for key, value := range vm.Tags {
			meta["tag."+key] = value
		}

		cfgMap := map[string]interface{}{
			"addr": privateIP,
			"note": renderAzureTemplate(noteTemplate, vm, resourceGroup, privateIP, publicIP, powerState),
		}
		if cfgMap["addr"] == "" {
			cfgMap["addr"] = publicIP
		}
		for _, key := range includeTags {
			if value := vm.Tags[key]; value != "" {
				cfgMap["tag_"+strings.ToLower(key)] = value
			}
		}

		name := renderAzureTemplate(nameTemplate, vm, resourceGroup, privateIP, publicIP, powerState)
		if name == "" {
			name = "azure:" + vm.Name
		}

		out = append(out, providerapi.InventoryServer{
			Name:   name,
			Config: cfgMap,
			Meta:   meta,
		})
	}

	return out, nil
}

type azureInventoryFilter struct {
	Statuses map[string]bool
}

func azureInventoryFilterFromConfig(config map[string]interface{}) azureInventoryFilter {
	statuses := normalizedStringSet(providerbuiltin.StringSlice(config, "statuses"))
	if len(statuses) == 0 {
		statuses = map[string]bool{"running": true}
		if azureBool(config, "include_stopped") {
			statuses["stopped"] = true
			statuses["deallocated"] = true
			statuses["stopping"] = true
			statuses["starting"] = true
		}
	}
	return azureInventoryFilter{Statuses: statuses}
}

func (f azureInventoryFilter) matches(powerState string) bool {
	if len(f.Statuses) == 0 {
		return true
	}
	return f.Statuses[strings.ToLower(powerState)]
}

func azureHealthCheck(config map[string]interface{}) (providerapi.HealthCheckResult, error) {
	client, err := newAzureClient(context.Background(), config)
	if err != nil {
		return providerapi.HealthCheckResult{}, err
	}
	if _, err := client.listVMs(config, true); err != nil {
		return providerapi.HealthCheckResult{}, err
	}
	return providerapi.HealthCheckResult{
		OK:      true,
		Message: "azure inventory provider can access Compute and Network resources",
	}, nil
}

func newAzureClient(ctx context.Context, config map[string]interface{}) (*azureClient, error) {
	baseURL := strings.TrimRight(providerbuiltin.String(config, "endpoint"), "/")
	if baseURL == "" {
		baseURL = "https://management.azure.com"
	}
	token, err := azureAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}
	return &azureClient{
		baseURL:       baseURL,
		httpClient:    &http.Client{},
		token:         token,
		nicCache:      map[string]azureNIC{},
		publicIPCache: map[string]string{},
	}, nil
}

func azureAccessToken(ctx context.Context, config map[string]interface{}) (string, error) {
	if token, err := providerbuiltin.ResolveConfigValue(config, "access_token"); err != nil {
		return "", err
	} else if token != "" {
		return token, nil
	}

	tenantID, err := providerbuiltin.ResolveConfigValue(config, "tenant_id")
	if err != nil {
		return "", err
	}
	clientID, err := providerbuiltin.ResolveConfigValue(config, "client_id")
	if err != nil {
		return "", err
	}
	clientSecret, err := providerbuiltin.ResolveConfigValue(config, "client_secret")
	if err != nil {
		return "", err
	}
	if tenantID == "" || clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("either access_token or tenant_id/client_id/client_secret is required")
	}

	authorityHost := strings.TrimRight(providerbuiltin.String(config, "authority_host"), "/")
	if authorityHost == "" {
		authorityHost = "https://login.microsoftonline.com"
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("scope", "https://management.azure.com//.default")

	tokenURL := fmt.Sprintf("%s/%s/oauth2/v2.0/token", authorityHost, tenantID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("azure token request failed: status=%s body=%s", resp.Status, string(body))
	}

	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("azure token response did not include access_token")
	}
	return payload.AccessToken, nil
}

func (c *azureClient) listVMs(config map[string]interface{}, statusOnly bool) ([]azureVM, error) {
	subscriptionID := providerbuiltin.String(config, "subscription_id")
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription_id is required")
	}
	resourceGroup := providerbuiltin.String(config, "resource_group")

	path := fmt.Sprintf("%s/subscriptions/%s/providers/Microsoft.Compute/virtualMachines?api-version=%s", c.baseURL, url.PathEscape(subscriptionID), azureComputeAPIVersion)
	if resourceGroup != "" {
		path = fmt.Sprintf("%s/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachines?api-version=%s", c.baseURL, url.PathEscape(subscriptionID), url.PathEscape(resourceGroup), azureComputeAPIVersion)
	}
	if statusOnly {
		path += "&statusOnly=true"
	}

	var out []azureVM
	nextURL := path
	for nextURL != "" {
		var payload azureVMListResponse
		if err := c.getJSON(nextURL, &payload); err != nil {
			return nil, err
		}
		out = append(out, payload.Value...)
		nextURL = payload.NextLink
	}
	return out, nil
}

func (c *azureClient) vmIPs(vm azureVM) (string, string, error) {
	for _, nicRef := range vm.Properties.NetworkProfile.NetworkInterfaces {
		if nicRef.ID == "" {
			continue
		}
		nic, err := c.getNIC(nicRef.ID)
		if err != nil {
			return "", "", err
		}
		for _, ipconf := range nic.Properties.IPConfigurations {
			privateIP := ipconf.Properties.PrivateIPAddress
			publicIP := ""
			if publicID := ipconf.Properties.PublicIPAddress.ID; publicID != "" {
				publicIP, err = c.getPublicIP(publicID)
				if err != nil {
					return privateIP, "", err
				}
			}
			if privateIP != "" || publicIP != "" {
				return privateIP, publicIP, nil
			}
		}
	}
	return "", "", nil
}

func (c *azureClient) getNIC(resourceID string) (azureNIC, error) {
	if nic, ok := c.nicCache[resourceID]; ok {
		return nic, nil
	}
	var nic azureNIC
	if err := c.getJSON(c.resourceURL(resourceID, azureNetworkAPIVersion), &nic); err != nil {
		return azureNIC{}, err
	}
	c.nicCache[resourceID] = nic
	return nic, nil
}

func (c *azureClient) getPublicIP(resourceID string) (string, error) {
	if value, ok := c.publicIPCache[resourceID]; ok {
		return value, nil
	}
	var pip azurePublicIP
	if err := c.getJSON(c.resourceURL(resourceID, azureNetworkAPIVersion), &pip); err != nil {
		return "", err
	}
	c.publicIPCache[resourceID] = pip.Properties.IPAddress
	return pip.Properties.IPAddress, nil
}

func (c *azureClient) resourceURL(resourceID, apiVersion string) string {
	return fmt.Sprintf("%s%s?api-version=%s", c.baseURL, resourceID, apiVersion)
}

func (c *azureClient) getJSON(rawURL string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("azure api request failed: status=%s url=%s body=%s", resp.Status, rawURL, string(body))
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	return decoder.Decode(out)
}

func azurePowerStateMap(vms []azureVM) map[string]string {
	out := make(map[string]string, len(vms))
	for _, vm := range vms {
		if vm.ID == "" {
			continue
		}
		if state := azurePowerState(vm.Properties.InstanceView.Statuses); state != "" {
			out[strings.ToLower(vm.ID)] = state
		}
	}
	return out
}

func azurePowerState(statuses []azureStatus) string {
	for _, status := range statuses {
		if strings.HasPrefix(strings.ToLower(status.Code), "powerstate/") {
			return strings.ToLower(strings.TrimPrefix(status.Code, "PowerState/"))
		}
	}
	return ""
}

func azureResourceGroup(resourceID string) string {
	parts := strings.Split(resourceID, "/")
	for i := 0; i+1 < len(parts); i++ {
		if strings.EqualFold(parts[i], "resourceGroups") {
			return parts[i+1]
		}
	}
	return ""
}

func renderAzureTemplate(template string, vm azureVM, resourceGroup, privateIP, publicIP, powerState string) string {
	if template == "" {
		return ""
	}
	replacerArgs := []string{
		"${id}", vm.ID,
		"${name}", vm.Name,
		"${location}", vm.Location,
		"${resource_group}", resourceGroup,
		"${private_ip}", privateIP,
		"${public_ip}", publicIP,
		"${power_state}", powerState,
		"${os_type}", strings.ToLower(vm.Properties.StorageProfile.OSDisk.OSType),
	}
	for key, value := range vm.Tags {
		replacerArgs = append(replacerArgs, "${tags."+key+"}", value)
	}
	return strings.NewReplacer(replacerArgs...).Replace(template)
}

func azureBool(config map[string]interface{}, key string) bool {
	switch v := config[key].(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

func normalizedStringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		out[value] = true
	}
	return out
}
