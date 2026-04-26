package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/blacknon/lssh/providerapi"
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
			Name:         "provider-inventory-proxmox",
			Capabilities: []string{"inventory"},
			Methods:      []string{providerapi.MethodPluginDescribe, providerapi.MethodHealthCheck, providerapi.MethodInventoryList},
			ReservedKeys: []string{
				"host", "scheme", "port", "insecure",
				"token_id", "token_id_env", "token_id_source", "token_id_source_env",
				"token_secret", "token_secret_env", "token_secret_source", "token_secret_source_env",
				"username", "user", "password", "password_env", "password_source", "password_source_env",
				"server_name_template", "note_template", "addr_template", "node_addr_prefix",
				"include_stopped", "include_templates", "vm_types", "statuses", "os_families",
			},
			ProtocolVersion: providerapi.Version,
		}, nil)
	case providerapi.MethodInventoryList:
		var params providerapi.InventoryListParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}

		servers, err := listProxmox(params.Config)
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
		result, err := proxmoxHealthCheck(params.Config)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "health_check_failed", err.Error())
			os.Exit(1)
		}
		_ = providerapi.WriteResponse(req, result, nil)
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

type proxmoxClusterResource struct {
	Name     string `json:"name"`
	Node     string `json:"node"`
	Status   string `json:"status"`
	VMID     int    `json:"vmid"`
	Type     string `json:"type"`
	Template int    `json:"template"`
	ID       string `json:"id"`
}

type proxmoxInventoryFilter struct {
	IncludeTemplates bool
	Statuses         map[string]bool
	VMTypes          map[string]bool
	OSFamilies       map[string]bool
}

type proxmoxNodeResource struct {
	Node   string `json:"node"`
	Status string `json:"status"`
}

const proxmoxOSTypeLookupParallelism = 4

func listProxmox(config map[string]interface{}) ([]providerapi.InventoryServer, error) {
	baseURL, err := proxmoxBaseURL(config)
	if err != nil {
		return nil, err
	}
	client := proxmoxHTTPClient(config)

	headers, err := proxmoxAuthHeaders(client, baseURL, config)
	if err != nil {
		return nil, err
	}

	data, err := proxmoxAPIGet(client, baseURL, "/cluster/resources?type=vm", headers)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Data []proxmoxClusterResource `json:"data"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	nameTemplate := providerapi.String(config, "server_name_template")
	if nameTemplate == "" {
		nameTemplate = "pve:${node}:${name}"
	}
	noteTemplate := providerapi.String(config, "note_template")
	addrTemplate := providerapi.String(config, "addr_template")
	nodeAddrPrefix := providerapi.String(config, "node_addr_prefix")
	filter := proxmoxInventoryFilterFromConfig(config)
	nodeStatuses, err := proxmoxNodeStatuses(client, baseURL, headers)
	if err != nil {
		return nil, err
	}
	resourceOSTypes, err := proxmoxResourceOSTypes(client, baseURL, headers, payload.Data, nodeStatuses)
	if err != nil {
		return nil, err
	}

	out := make([]providerapi.InventoryServer, 0, len(payload.Data))
	for _, resource := range payload.Data {
		ostype := resourceOSTypes[resource.ID]
		osFamily := proxmoxOSFamily(resource.Type, ostype)
		if !filter.matches(resource, osFamily) {
			continue
		}

		addr := ""
		if addrTemplate != "" {
			addr = renderProxmoxTemplate(addrTemplate, resource.Name, resource.Node, resource.VMID)
		} else if nodeAddrPrefix != "" {
			addr = nodeAddrPrefix + resource.Node
		}
		out = append(out, providerapi.InventoryServer{
			Name: renderProxmoxTemplate(nameTemplate, resource.Name, resource.Node, resource.VMID),
			Config: map[string]interface{}{
				"addr": addr,
				"note": renderProxmoxTemplate(noteTemplate, resource.Name, resource.Node, resource.VMID),
			},
			Meta: map[string]string{
				"provider":  "proxmox",
				"plugin":    "provider-inventory-proxmox",
				"name":      resource.Name,
				"node":      resource.Node,
				"vmid":      fmt.Sprint(resource.VMID),
				"type":      resource.Type,
				"id":        resource.ID,
				"status":    resource.Status,
				"os_family": osFamily,
			},
		})
		if ostype != "" {
			out[len(out)-1].Meta["ostype"] = ostype
		}
	}

	return out, nil
}

func proxmoxInventoryFilterFromConfig(config map[string]interface{}) proxmoxInventoryFilter {
	filter := proxmoxInventoryFilter{
		IncludeTemplates: proxmoxBool(config, "include_templates"),
		Statuses:         stringSet(providerapi.StringSlice(config, "statuses")),
		VMTypes:          stringSet(providerapi.StringSlice(config, "vm_types")),
		OSFamilies:       normalizedStringSet(providerapi.StringSlice(config, "os_families")),
	}

	if len(filter.Statuses) == 0 {
		filter.Statuses = map[string]bool{"running": true}
		if proxmoxBool(config, "include_stopped") {
			filter.Statuses["stopped"] = true
		}
	}

	return filter
}

func (f proxmoxInventoryFilter) matches(resource proxmoxClusterResource, osFamily string) bool {
	if !f.IncludeTemplates && resource.Template != 0 {
		return false
	}
	if len(f.Statuses) > 0 && !f.Statuses[resource.Status] {
		return false
	}
	if len(f.VMTypes) > 0 && !f.VMTypes[resource.Type] {
		return false
	}
	if len(f.OSFamilies) > 0 && !f.OSFamilies[osFamily] {
		return false
	}
	return true
}

func proxmoxNodeStatuses(client *http.Client, baseURL *url.URL, headers http.Header) (map[string]string, error) {
	data, err := proxmoxAPIGet(client, baseURL, "/cluster/resources?type=node", headers)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Data []proxmoxNodeResource `json:"data"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	result := make(map[string]string, len(payload.Data))
	for _, resource := range payload.Data {
		if resource.Node == "" {
			continue
		}
		result[resource.Node] = resource.Status
	}
	return result, nil
}

func proxmoxResourceOSTypes(client *http.Client, baseURL *url.URL, headers http.Header, resources []proxmoxClusterResource, nodeStatuses map[string]string) (map[string]string, error) {
	return proxmoxResourceOSTypesWithLookup(resources, nodeStatuses, func(resource proxmoxClusterResource) (string, error) {
		return proxmoxQEMUGuestOSType(client, baseURL, headers, resource.Node, resource.VMID)
	})
}

func proxmoxResourceOSTypesWithLookup(resources []proxmoxClusterResource, nodeStatuses map[string]string, lookup func(proxmoxClusterResource) (string, error)) (map[string]string, error) {
	result := map[string]string{}
	var resultMu sync.Mutex
	var warningMu sync.Mutex
	warnings := make([]string, 0)
	sem := make(chan struct{}, proxmoxOSTypeLookupParallelism)
	var wg sync.WaitGroup

	for _, resource := range resources {
		if resource.Type != "qemu" {
			continue
		}
		if nodeStatuses[resource.Node] != "" && nodeStatuses[resource.Node] != "online" {
			warningMu.Lock()
			warnings = append(warnings, fmt.Sprintf("warning: skip ostype lookup for %s because node %s is %s", resource.ID, resource.Node, nodeStatuses[resource.Node]))
			warningMu.Unlock()
			continue
		}

		resource := resource
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ostype, err := lookup(resource)
			if err != nil {
				warningMu.Lock()
				warnings = append(warnings, fmt.Sprintf("warning: failed to get ostype for %s: %v", resource.ID, err))
				warningMu.Unlock()
				return
			}

			resultMu.Lock()
			result[resource.ID] = ostype
			resultMu.Unlock()
		}()
	}

	wg.Wait()
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, warning)
	}

	return result, nil
}

func proxmoxQEMUGuestOSType(client *http.Client, baseURL *url.URL, headers http.Header, node string, vmid int) (string, error) {
	data, err := proxmoxAPIGet(client, baseURL, fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmid), headers)
	if err != nil {
		return "", err
	}

	var payload struct {
		Data struct {
			OSType string `json:"ostype"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", err
	}
	return payload.Data.OSType, nil
}

func proxmoxOSFamily(resourceType, ostype string) string {
	if resourceType == "lxc" {
		return "non-windows"
	}
	if ostype == "" {
		return "unknown"
	}
	if strings.HasPrefix(strings.ToLower(ostype), "w") {
		return "windows"
	}
	return "non-windows"
}

func proxmoxBaseURL(config map[string]interface{}) (*url.URL, error) {
	host := providerapi.String(config, "host")
	if host == "" {
		return nil, fmt.Errorf("host is required")
	}

	scheme := providerapi.String(config, "scheme")
	if scheme == "" {
		scheme = "https"
	}

	port := providerapi.String(config, "port")
	if port == "" {
		port = "8006"
	}

	u, err := url.Parse(fmt.Sprintf("%s://%s", scheme, host))
	if err != nil {
		return nil, err
	}
	if u.Port() == "" && port != "" {
		u.Host = u.Hostname() + ":" + port
	}
	return u, nil
}

func proxmoxHTTPClient(config map[string]interface{}) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: proxmoxBool(config, "insecure"),
	}
	return &http.Client{Transport: transport}
}

func proxmoxAuthHeaders(client *http.Client, baseURL *url.URL, config map[string]interface{}) (http.Header, error) {
	tokenID, err := providerapi.ResolveConfigValue(config, "token_id")
	if err != nil {
		return nil, err
	}
	tokenSecret, err := providerapi.ResolveConfigValue(config, "token_secret")
	if err != nil {
		return nil, err
	}
	if tokenID != "" || tokenSecret != "" {
		if tokenID == "" || tokenSecret == "" {
			return nil, fmt.Errorf("token_id and token_secret must both be set")
		}
		headers := make(http.Header)
		headers.Set("Authorization", "PVEAPIToken="+tokenID+"="+tokenSecret)
		return headers, nil
	}

	username := providerapi.String(config, "username")
	if username == "" {
		username = providerapi.String(config, "user")
	}
	password, err := providerapi.ResolveConfigValue(config, "password")
	if err != nil {
		return nil, err
	}
	if username == "" || password == "" {
		return nil, fmt.Errorf("either token_id/token_secret or username/password is required")
	}

	form := url.Values{}
	form.Set("username", username)
	form.Set("password", password)

	ticketURL := *baseURL
	ticketURL.Path = "/api2/json/access/ticket"
	req, err := http.NewRequest(http.MethodPost, ticketURL.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, proxmoxHTTPError("proxmox auth failed", resp.Status, ticketURL.Path, body)
	}

	var payload struct {
		Data struct {
			Ticket              string `json:"ticket"`
			CSRFPreventionToken string `json:"CSRFPreventionToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if payload.Data.Ticket == "" {
		return nil, fmt.Errorf("proxmox auth failed: no ticket returned")
	}

	headers := make(http.Header)
	headers.Set("Cookie", "PVEAuthCookie="+payload.Data.Ticket)
	if payload.Data.CSRFPreventionToken != "" {
		headers.Set("CSRFPreventionToken", payload.Data.CSRFPreventionToken)
	}
	return headers, nil
}

func proxmoxAPIGet(client *http.Client, baseURL *url.URL, path string, headers http.Header) ([]byte, error) {
	u := *baseURL
	endpointPath := strings.TrimPrefix(path, "/")
	if idx := strings.Index(endpointPath, "?"); idx >= 0 {
		u.Path = "/api2/json/" + endpointPath[:idx]
		u.RawQuery = endpointPath[idx+1:]
	} else {
		u.Path = "/api2/json/" + endpointPath
		u.RawQuery = ""
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, proxmoxHTTPError("proxmox api request failed", resp.Status, proxmoxRequestPath(u.Path, u.RawQuery), body)
	}
	return body, nil
}

func proxmoxHTTPError(prefix, status, requestPath string, body []byte) error {
	trimmedBody := strings.TrimSpace(string(body))
	if trimmedBody == "" {
		return fmt.Errorf("%s: status=%s path=%s", prefix, status, requestPath)
	}
	return fmt.Errorf("%s: status=%s path=%s body=%s", prefix, status, requestPath, trimmedBody)
}

func proxmoxRequestPath(path, rawQuery string) string {
	if rawQuery == "" {
		return path
	}
	return path + "?" + rawQuery
}

func proxmoxBool(config map[string]interface{}, key string) bool {
	raw, ok := config[key]
	if !ok || raw == nil {
		return false
	}
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(v)
		if err == nil {
			return parsed
		}
	}
	return false
}

func proxmoxHealthCheck(config map[string]interface{}) (providerapi.HealthCheckResult, error) {
	baseURL, err := proxmoxBaseURL(config)
	if err != nil {
		return providerapi.HealthCheckResult{}, err
	}

	client := proxmoxHTTPClient(config)
	headers, err := proxmoxAuthHeaders(client, baseURL, config)
	if err != nil {
		return providerapi.HealthCheckResult{}, err
	}

	if _, err := proxmoxAPIGet(client, baseURL, "/version", headers); err != nil {
		return providerapi.HealthCheckResult{}, err
	}

	return providerapi.HealthCheckResult{
		OK:      true,
		Message: "proxmox inventory provider can access the API",
	}, nil
}

func stringSet(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func normalizedStringSet(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[strings.ToLower(value)] = true
	}
	return out
}

func renderProxmoxTemplate(template, name, node string, vmid int) string {
	if template == "" {
		return ""
	}
	return strings.NewReplacer(
		"${name}", name,
		"${node}", node,
		"${vmid}", fmt.Sprint(vmid),
	).Replace(template)
}
