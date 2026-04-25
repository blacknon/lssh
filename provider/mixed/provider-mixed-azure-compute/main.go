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
	"runtime"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/blacknon/lssh/providerapi"
)

const (
	azureComputeAPIVersion = "2025-04-01"
	azureNetworkAPIVersion = "2024-07-01"
)

func main() {
	req, err := providerapi.ReadRequest()
	if err != nil {
		_ = providerapi.WriteError(err.Error())
		os.Exit(1)
	}

	switch req.Method {
	case providerapi.MethodPluginDescribe:
		_ = providerapi.WriteResponse(req, providerapi.PluginDescribeResult{
			Name:           "provider-mixed-azure-compute",
			Capabilities:   []string{"inventory", "connector"},
			ConnectorNames: []string{"azure-bastion"},
			Methods:        []string{providerapi.MethodPluginDescribe, providerapi.MethodHealthCheck, providerapi.MethodInventoryList, providerapi.MethodConnectorDescribe, providerapi.MethodConnectorPrepare, providerapi.MethodConnectorDial},
			ReservedKeys: []string{
				"subscription_id",
				"username", "user",
				"tenant_id", "tenant_id_env", "tenant_id_source", "tenant_id_source_env",
				"client_id", "client_id_env", "client_id_source", "client_id_source_env",
				"client_secret", "client_secret_env", "client_secret_source", "client_secret_source_env",
				"access_token", "access_token_env", "access_token_source", "access_token_source_env",
				"authority_host", "endpoint", "resource_group",
				"statuses", "include_stopped", "include_tags",
				"server_name_template", "note_template",
				"bastion_runtime", "bastion_name", "bastion_resource_group", "bastion_auth_type",
			},
			ProtocolVersion: providerapi.Version,
		}, nil)
	case providerapi.MethodInventoryList:
		var params providerapi.InventoryListParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		servers, err := listAzure(params.Config)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "inventory_failed", err.Error())
			os.Exit(1)
		}
		_ = providerapi.WriteResponse(req, providerapi.InventoryListResult{Servers: servers}, nil)
	case providerapi.MethodHealthCheck:
		var params providerapi.HealthCheckParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := azureHealthCheck(params.Config)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "health_check_failed", err.Error())
			os.Exit(1)
		}
		_ = providerapi.WriteResponse(req, result, nil)
	case providerapi.MethodConnectorDescribe:
		var params providerapi.ConnectorDescribeParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := azureConnectorDescribe(params)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "connector_describe_failed", err.Error())
			os.Exit(1)
		}
		_ = providerapi.WriteResponse(req, result, nil)
	case providerapi.MethodConnectorPrepare, providerapi.MethodTransportPrep:
		var params providerapi.ConnectorPrepareParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := azureConnectorPrepare(params)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "connector_prepare_failed", err.Error())
			os.Exit(1)
		}
		_ = providerapi.WriteResponse(req, result, nil)
	case providerapi.MethodConnectorDial:
		var params providerapi.ConnectorRuntimeParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		if err := azureConnectorRunDial(params); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		_ = providerapi.WriteErrorResponse(req, "unsupported_method", fmt.Sprintf("unsupported method %q", req.Method))
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
	baseURL        string
	httpClient     *http.Client
	token          string
	subscriptionID string

	cacheMu       sync.RWMutex
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

	nameTemplate := providerapi.String(config, "server_name_template")
	if nameTemplate == "" {
		nameTemplate = "azure:${name}"
	}
	noteTemplate := providerapi.String(config, "note_template")
	addrStrategy := azureAddrStrategy(config)
	includeTags := providerapi.StringSlice(config, "include_tags")
	filter := azureInventoryFilterFromConfig(config)

	entries, err := client.buildInventoryEntries(vms, powerStateByID, filter, nameTemplate, noteTemplate, addrStrategy, includeTags)
	if err != nil {
		return nil, err
	}
	out := make([]providerapi.InventoryServer, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		out = append(out, *entry)
	}
	return out, nil
}

func (c *azureClient) buildInventoryEntries(vms []azureVM, powerStateByID map[string]string, filter azureInventoryFilter, nameTemplate, noteTemplate, addrStrategy string, includeTags []string) ([]*providerapi.InventoryServer, error) {
	results := make([]*providerapi.InventoryServer, len(vms))
	errCh := make(chan error, 1)
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup

	for i, vm := range vms {
		i := i
		vm := vm
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			server, warn, err := c.buildInventoryEntry(vm, powerStateByID, filter, nameTemplate, noteTemplate, addrStrategy, includeTags)
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			if warn != "" {
				fmt.Fprintln(os.Stderr, warn)
			}
			results[i] = server
		}()
	}

	wg.Wait()
	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	return results, nil
}

func (c *azureClient) buildInventoryEntry(vm azureVM, powerStateByID map[string]string, filter azureInventoryFilter, nameTemplate, noteTemplate, addrStrategy string, includeTags []string) (*providerapi.InventoryServer, string, error) {
	powerState := powerStateByID[strings.ToLower(vm.ID)]
	if powerState == "" {
		powerState = azurePowerState(vm.Properties.InstanceView.Statuses)
	}
	if powerState == "" && vm.ID != "" {
		statuses, err := c.getVMInstanceViewStatuses(vm.ID)
		if err != nil {
			return nil, "", err
		}
		powerState = azurePowerState(statuses)
	}
	if !filter.matches(powerState) {
		return nil, "", nil
	}

	privateIP, publicIP, ipErr := c.vmIPs(vm)
	warn := ""
	if ipErr != nil {
		warn = fmt.Sprintf("warning: failed to resolve VM network for %s: %v", vm.Name, ipErr)
	}

	resourceGroup := azureResourceGroup(vm.ID)
	osType := strings.ToLower(vm.Properties.StorageProfile.OSDisk.OSType)
	meta := map[string]string{
		"provider":           "azure",
		"plugin":             "provider-mixed-azure-compute",
		"id":                 vm.ID,
		"name":               vm.Name,
		"subscription_id":    c.subscriptionID,
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

	addr := azureSelectAddress(privateIP, publicIP, addrStrategy)
	cfgMap := map[string]interface{}{
		"addr": addr,
		"note": renderAzureTemplate(noteTemplate, vm, resourceGroup, privateIP, publicIP, powerState),
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

	return &providerapi.InventoryServer{
		Name:   name,
		Config: cfgMap,
		Meta:   meta,
	}, warn, nil
}

type azureInventoryFilter struct {
	Statuses map[string]bool
}

func azureInventoryFilterFromConfig(config map[string]interface{}) azureInventoryFilter {
	statuses := normalizedStringSet(providerapi.StringSlice(config, "statuses"))
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

func azureAddrStrategy(config map[string]interface{}) string {
	switch strings.TrimSpace(strings.ToLower(providerapi.String(config, "addr_strategy"))) {
	case "", "private_first":
		return "private_first"
	case "public_first":
		return "public_first"
	case "private_only":
		return "private_only"
	case "public_only":
		return "public_only"
	default:
		return "private_first"
	}
}

func azureSelectAddress(privateIP, publicIP, strategy string) string {
	switch strategy {
	case "public_first":
		if publicIP != "" {
			return publicIP
		}
		return privateIP
	case "private_only":
		return privateIP
	case "public_only":
		return publicIP
	default:
		if privateIP != "" {
			return privateIP
		}
		return publicIP
	}
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

type azureBastionConnectorConfig struct {
	BastionName      string
	BastionGroup     string
	TargetResourceID string
	SubscriptionID   string
	User             string
	KeyFile          string
	Port             string
}

func azureConnectorDescribe(params providerapi.ConnectorDescribeParams) (providerapi.ConnectorDescribeResult, error) {
	cfg, err := azureConnectorConfig(params.Config, params.Target)
	if err != nil {
		return providerapi.ConnectorDescribeResult{}, err
	}

	commandRuntime := azureConnectorRuntime(params.Config, params.Target.Config) == "command"
	sdkRuntime := !commandRuntime
	sdkReason := ""
	if !sdkRuntime {
		sdkReason = "azure-bastion sdk runtime is disabled by configuration"
	}
	mountSupported := sdkRuntime && (runtime.GOOS == "linux" || runtime.GOOS == "darwin")
	mountReason := sdkReason
	if sdkRuntime && !mountSupported {
		mountReason = "mount via azure-bastion sdk runtime currently supports linux and macos only"
	}

	return providerapi.ConnectorDescribeResult{
		Capabilities: map[string]providerapi.ConnectorCapability{
			"shell": {
				Supported: cfg.TargetResourceID != "",
				Reason:    unsupportedReason(cfg.TargetResourceID != "", "azure-bastion requires target_resource_id metadata"),
				Preferred: sdkRuntime,
			},
			"exec": {
				Supported: sdkRuntime && cfg.TargetResourceID != "",
				Reason:    unsupportedReason(sdkRuntime && cfg.TargetResourceID != "", "azure-bastion exec is available only in sdk runtime"),
			},
			"exec_pty": {
				Supported: sdkRuntime && cfg.TargetResourceID != "",
				Reason:    unsupportedReason(sdkRuntime && cfg.TargetResourceID != "", "azure-bastion exec_pty is available only in sdk runtime"),
			},
			"sftp_transport": {
				Supported: sdkRuntime && cfg.TargetResourceID != "",
				Reason:    unsupportedReason(sdkRuntime && cfg.TargetResourceID != "", sdkReason),
			},
			"upload": {
				Supported: sdkRuntime && cfg.TargetResourceID != "",
				Reason:    unsupportedReason(sdkRuntime && cfg.TargetResourceID != "", sdkReason),
				Requires:  []string{"sftp_transport"},
			},
			"download": {
				Supported: sdkRuntime && cfg.TargetResourceID != "",
				Reason:    unsupportedReason(sdkRuntime && cfg.TargetResourceID != "", sdkReason),
				Requires:  []string{"sftp_transport"},
			},
			"mount": {
				Supported: mountSupported && cfg.TargetResourceID != "",
				Reason:    unsupportedReason(mountSupported && cfg.TargetResourceID != "", mountReason),
				Requires:  []string{"sftp_transport"},
			},
			"port_forward_local": {
				Supported: cfg.TargetResourceID != "",
				Reason:    unsupportedReason(cfg.TargetResourceID != "", "azure-bastion requires target_resource_id metadata"),
			},
			"tcp_dial_transport": {
				Supported: sdkRuntime && cfg.TargetResourceID != "",
				Reason:    unsupportedReason(sdkRuntime && cfg.TargetResourceID != "", sdkReason),
			},
			"port_forward_remote": {
				Supported: false,
				Reason:    "azure-bastion does not support remote port forwarding in the first connector wave",
			},
			"agent_forward": {
				Supported: false,
				Reason:    "azure-bastion does not provide agent forwarding in the first connector wave",
			},
		},
	}, nil
}

func azureConnectorPrepare(params providerapi.ConnectorPrepareParams) (providerapi.ConnectorPrepareResult, error) {
	cfg, err := azureConnectorConfig(params.Config, params.Target)
	if err != nil {
		return providerapi.ConnectorPrepareResult{}, err
	}
	if azureConnectorRuntime(params.Config, params.Target.Config) == "command" {
		return azureConnectorPrepareCommand(params, cfg)
	}
	return azureConnectorPrepareSDK(params, cfg), nil
}

func azureConnectorConfig(config map[string]interface{}, target providerapi.ConnectorTarget) (azureBastionConnectorConfig, error) {
	cfg := azureBastionConnectorConfig{
		BastionName:      firstNonEmpty(providerapi.String(target.Config, "bastion_name"), providerapi.String(config, "bastion_name")),
		BastionGroup:     firstNonEmpty(providerapi.String(target.Config, "bastion_resource_group"), providerapi.String(config, "bastion_resource_group"), target.Meta["resource_group"]),
		TargetResourceID: firstNonEmpty(target.Meta["id"], providerapi.String(target.Config, "target_resource_id")),
		SubscriptionID:   firstNonEmpty(target.Meta["subscription_id"], providerapi.String(target.Config, "subscription_id"), providerapi.String(config, "subscription_id")),
		User:             providerapi.String(target.Config, "user"),
		KeyFile:          providerapi.ExpandPath(providerapi.String(target.Config, "key")),
		Port:             firstNonEmpty(providerapi.String(target.Config, "port"), "22"),
	}
	if cfg.BastionName == "" {
		return cfg, fmt.Errorf("bastion_name is required for azure-bastion connector")
	}
	if cfg.BastionGroup == "" {
		return cfg, fmt.Errorf("bastion_resource_group is required for azure-bastion connector")
	}
	if cfg.TargetResourceID == "" {
		return cfg, fmt.Errorf("target_resource_id metadata is required for azure-bastion connector")
	}
	return cfg, nil
}

func azureConnectorPrepareSDK(params providerapi.ConnectorPrepareParams, cfg azureBastionConnectorConfig) providerapi.ConnectorPrepareResult {
	targetPort := firstNonEmpty(azureOptionString(params.Operation.Options, "target_port"), cfg.Port)
	return providerapi.ConnectorPrepareResult{
		Supported: true,
		Plan: providerapi.ConnectorPlan{
			Kind: "provider-managed",
			Details: map[string]interface{}{
				"connector":              "azure-bastion",
				"ssh_runtime":            "sdk",
				"transport":              "ssh_transport",
				"operation":              params.Operation.Name,
				"bastion_name":           cfg.BastionName,
				"bastion_resource_group": cfg.BastionGroup,
				"target_resource_id":     cfg.TargetResourceID,
				"subscription_id":        cfg.SubscriptionID,
				"target_port":            targetPort,
			},
		},
	}
}

func azureConnectorPrepareCommand(params providerapi.ConnectorPrepareParams, cfg azureBastionConnectorConfig) (providerapi.ConnectorPrepareResult, error) {
	args := []string{
		"network", "bastion", "ssh",
		"--name", cfg.BastionName,
		"--resource-group", cfg.BastionGroup,
		"--target-resource-id", cfg.TargetResourceID,
	}
	if cfg.SubscriptionID != "" {
		args = append(args, "--subscription", cfg.SubscriptionID)
	}
	if cfg.User != "" {
		args = append(args, "--username", cfg.User)
	}
	if cfg.KeyFile != "" {
		args = append(args, "--auth-type", "ssh-key", "--ssh-key", cfg.KeyFile)
	} else {
		args = append(args, "--auth-type", firstNonEmpty(providerapi.String(params.Target.Config, "bastion_auth_type"), providerapi.String(params.Config, "bastion_auth_type"), "AAD"))
	}

	switch params.Operation.Name {
	case "shell":
		return providerapi.ConnectorPrepareResult{
			Supported: true,
			Plan: providerapi.ConnectorPlan{
				Kind:    "command",
				Program: "az",
				Args:    args,
				Details: map[string]interface{}{"connector": "azure-bastion", "operation": "shell", "runtime": "command"},
			},
		}, nil
	case "port_forward_local":
		listenPort := azureOptionString(params.Operation.Options, "listen_port")
		targetHost := azureOptionString(params.Operation.Options, "target_host")
		targetPort := firstNonEmpty(azureOptionString(params.Operation.Options, "target_port"), cfg.Port)
		if listenPort == "" || targetHost == "" || targetPort == "" {
			return providerapi.ConnectorPrepareResult{}, fmt.Errorf("azure-bastion local forward requires listen_port, target_host, and target_port")
		}
		commandArgs := append([]string{}, args...)
		commandArgs = append(commandArgs, "--", "-N", "-L", fmt.Sprintf("%s:%s:%s", listenPort, targetHost, targetPort))
		return providerapi.ConnectorPrepareResult{
			Supported: true,
			Plan: providerapi.ConnectorPlan{
				Kind:    "command",
				Program: "az",
				Args:    commandArgs,
				Details: map[string]interface{}{"connector": "azure-bastion", "operation": "port_forward_local", "runtime": "command"},
			},
		}, nil
	default:
		return providerapi.ConnectorPrepareResult{
			Supported: false,
			Plan: providerapi.ConnectorPlan{
				Kind: "command",
				Details: map[string]interface{}{
					"connector": "azure-bastion",
					"reason":    fmt.Sprintf("operation %q is not supported by the azure-bastion command runtime", params.Operation.Name),
				},
			},
		}, nil
	}
}

func azureConnectorRuntime(config map[string]interface{}, targetConfig map[string]interface{}) string {
	switch strings.ToLower(strings.TrimSpace(firstNonEmpty(
		providerapi.String(targetConfig, "bastion_runtime"),
		providerapi.String(config, "bastion_runtime"),
	))) {
	case "", "sdk":
		return "sdk"
	case "command":
		return "command"
	default:
		return "sdk"
	}
}

func azureOptionString(raw map[string]interface{}, key string) string {
	if raw == nil {
		return ""
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func newAzureClient(ctx context.Context, config map[string]interface{}) (*azureClient, error) {
	baseURL := strings.TrimRight(providerapi.String(config, "endpoint"), "/")
	if baseURL == "" {
		baseURL = "https://management.azure.com"
	}
	token, err := azureAccessToken(ctx, config)
	if err != nil {
		return nil, err
	}
	subscriptionID, err := azureSubscriptionID(ctx, config, baseURL, token)
	if err != nil {
		return nil, err
	}
	return &azureClient{
		baseURL:        baseURL,
		httpClient:     &http.Client{},
		token:          token,
		subscriptionID: subscriptionID,
		nicCache:       map[string]azureNIC{},
		publicIPCache:  map[string]string{},
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func unsupportedReason(supported bool, reason string) string {
	if supported {
		return ""
	}
	return reason
}

func azureAccessToken(ctx context.Context, config map[string]interface{}) (string, error) {
	if token, err := providerapi.ResolveConfigValue(config, "access_token"); err != nil {
		return "", err
	} else if token != "" {
		return token, nil
	}

	scope := azureTokenScope(config)
	tenantID, err := providerapi.ResolveConfigValue(config, "tenant_id")
	if err != nil {
		return "", err
	}
	clientID, err := providerapi.ResolveConfigValue(config, "client_id")
	if err != nil {
		return "", err
	}
	clientSecret, err := providerapi.ResolveConfigValue(config, "client_secret")
	if err != nil {
		return "", err
	}
	if tenantID != "" || clientID != "" || clientSecret != "" {
		if tenantID == "" || clientID == "" || clientSecret == "" {
			return "", fmt.Errorf("tenant_id, client_id, and client_secret must all be set together")
		}
		credential, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
		if err != nil {
			return "", err
		}
		token, err := credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{scope}})
		if err != nil {
			return "", err
		}
		if token.Token == "" {
			return "", fmt.Errorf("azure sdk client secret credential did not return an access token")
		}
		return token.Token, nil
	}

	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return "", err
	}
	token, err := credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{scope}})
	if err != nil {
		return "", fmt.Errorf("default azure credential token acquisition failed: %w", err)
	}
	if token.Token == "" {
		return "", fmt.Errorf("default azure credential did not return an access token")
	}
	return token.Token, nil
}

func azureTokenScope(config map[string]interface{}) string {
	endpoint := strings.TrimRight(providerapi.String(config, "endpoint"), "/")
	if endpoint == "" {
		endpoint = "https://management.azure.com"
	}
	return endpoint + "/.default"
}

func azureSubscriptionID(ctx context.Context, config map[string]interface{}, baseURL, token string) (string, error) {
	if subscriptionID, err := providerapi.ResolveConfigValue(config, "subscription_id"); err != nil {
		return "", err
	} else if subscriptionID != "" {
		return subscriptionID, nil
	}
	if subscriptionID := strings.TrimSpace(os.Getenv("AZURE_SUBSCRIPTION_ID")); subscriptionID != "" {
		return subscriptionID, nil
	}
	if token == "" {
		return "", fmt.Errorf("subscription_id is required")
	}
	return azureDiscoverSubscriptionID(ctx, baseURL, token)
}

func azureDiscoverSubscriptionID(ctx context.Context, baseURL, token string) (string, error) {
	subscriptionsURL := strings.TrimRight(baseURL, "/") + "/subscriptions?api-version=2022-12-01"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, subscriptionsURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

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
		return "", fmt.Errorf("azure subscription discovery failed: status=%s body=%s", resp.Status, string(body))
	}

	var payload struct {
		Value []struct {
			SubscriptionID string `json:"subscriptionId"`
			State          string `json:"state"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}

	selected, err := azureSelectSubscriptionID(payload.Value)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stderr, "info: azure auto-selected subscription_id=%s\n", selected)
	return selected, nil
}

func azureSelectSubscriptionID(subscriptions []struct {
	SubscriptionID string `json:"subscriptionId"`
	State          string `json:"state"`
}) (string, error) {
	enabled := []string{}
	for _, sub := range subscriptions {
		if strings.TrimSpace(sub.SubscriptionID) == "" {
			continue
		}
		if sub.State == "" || strings.EqualFold(sub.State, "Enabled") {
			enabled = append(enabled, sub.SubscriptionID)
		}
	}

	switch len(enabled) {
	case 0:
		return "", fmt.Errorf("subscription_id is required: Azure did not return any enabled subscriptions")
	case 1:
		return enabled[0], nil
	default:
		return "", fmt.Errorf("subscription_id is required: multiple enabled subscriptions are available; set subscription_id, subscription_id_env, subscription_id_source, or AZURE_SUBSCRIPTION_ID")
	}
}

func (c *azureClient) listVMs(config map[string]interface{}, statusOnly bool) ([]azureVM, error) {
	subscriptionID := c.subscriptionID
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription_id is required")
	}
	resourceGroup := providerapi.String(config, "resource_group")
	path := azureVMListPath(c.baseURL, subscriptionID, resourceGroup, statusOnly)

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

func azureVMListPath(baseURL, subscriptionID, resourceGroup string, statusOnly bool) string {
	if resourceGroup != "" {
		return fmt.Sprintf("%s/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachines?api-version=%s",
			baseURL,
			url.PathEscape(subscriptionID),
			url.PathEscape(resourceGroup),
			azureComputeAPIVersion,
		)
	}

	path := fmt.Sprintf("%s/subscriptions/%s/providers/Microsoft.Compute/virtualMachines?api-version=%s",
		baseURL,
		url.PathEscape(subscriptionID),
		azureComputeAPIVersion,
	)
	if statusOnly {
		return path + "&statusOnly=true"
	}
	return path
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
	c.cacheMu.RLock()
	if nic, ok := c.nicCache[resourceID]; ok {
		c.cacheMu.RUnlock()
		return nic, nil
	}
	c.cacheMu.RUnlock()
	var nic azureNIC
	if err := c.getJSON(c.resourceURL(resourceID, azureNetworkAPIVersion), &nic); err != nil {
		return azureNIC{}, err
	}
	c.cacheMu.Lock()
	c.nicCache[resourceID] = nic
	c.cacheMu.Unlock()
	return nic, nil
}

func (c *azureClient) getPublicIP(resourceID string) (string, error) {
	c.cacheMu.RLock()
	if value, ok := c.publicIPCache[resourceID]; ok {
		c.cacheMu.RUnlock()
		return value, nil
	}
	c.cacheMu.RUnlock()
	var pip azurePublicIP
	if err := c.getJSON(c.resourceURL(resourceID, azureNetworkAPIVersion), &pip); err != nil {
		return "", err
	}
	c.cacheMu.Lock()
	c.publicIPCache[resourceID] = pip.Properties.IPAddress
	c.cacheMu.Unlock()
	return pip.Properties.IPAddress, nil
}

func (c *azureClient) resourceURL(resourceID, apiVersion string) string {
	return fmt.Sprintf("%s%s?api-version=%s", c.baseURL, resourceID, apiVersion)
}

func (c *azureClient) getVMInstanceViewStatuses(resourceID string) ([]azureStatus, error) {
	var payload struct {
		Statuses []azureStatus `json:"statuses"`
	}
	if err := c.getJSON(c.resourceURL(resourceID+"/instanceView", azureComputeAPIVersion), &payload); err != nil {
		return nil, err
	}
	return payload.Statuses, nil
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
