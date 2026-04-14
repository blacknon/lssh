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

	"github.com/blacknon/lssh/internal/providerapi"
	"github.com/blacknon/lssh/internal/providerbuiltin"
)

func main() {
	req, err := providerbuiltin.ReadRequest()
	if err != nil {
		_ = providerbuiltin.WriteError(err.Error())
		os.Exit(1)
	}

	switch req.Method {
	case providerapi.MethodInventoryList:
		var params providerapi.InventoryListParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteError(err.Error())
			os.Exit(1)
		}

		servers, err := listProxmox(params.Config)
		if err != nil {
			_ = providerbuiltin.WriteError(err.Error())
			os.Exit(1)
		}
		_ = providerbuiltin.WriteResult(providerapi.InventoryListResult{Servers: servers})
	default:
		_ = providerbuiltin.WriteError(fmt.Sprintf("unsupported method %q", req.Method))
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
		Data []struct {
			Name     string `json:"name"`
			Node     string `json:"node"`
			Status   string `json:"status"`
			VMID     int    `json:"vmid"`
			Type     string `json:"type"`
			Template int    `json:"template"`
			ID       string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	nameTemplate := providerbuiltin.String(config, "server_name_template")
	if nameTemplate == "" {
		nameTemplate = "pve:${node}:${name}"
	}
	noteTemplate := providerbuiltin.String(config, "note_template")
	addrTemplate := providerbuiltin.String(config, "addr_template")
	nodeAddrPrefix := providerbuiltin.String(config, "node_addr_prefix")
	includeStopped := proxmoxBool(config, "include_stopped")
	includeTemplates := proxmoxBool(config, "include_templates")
	vmTypes := stringSet(providerbuiltin.StringSlice(config, "vm_types"))

	out := make([]providerapi.InventoryServer, 0, len(payload.Data))
	for _, resource := range payload.Data {
		if !includeStopped && resource.Status != "running" {
			continue
		}
		if !includeTemplates && resource.Template != 0 {
			continue
		}
		if len(vmTypes) > 0 && !vmTypes[resource.Type] {
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
				"provider": "proxmox",
				"plugin":   "provider-inventory-proxmox",
				"name":     resource.Name,
				"node":     resource.Node,
				"vmid":     fmt.Sprint(resource.VMID),
				"type":     resource.Type,
				"id":       resource.ID,
				"status":   resource.Status,
			},
		})
	}

	return out, nil
}

func proxmoxBaseURL(config map[string]interface{}) (*url.URL, error) {
	host := providerbuiltin.String(config, "host")
	if host == "" {
		return nil, fmt.Errorf("host is required")
	}

	scheme := providerbuiltin.String(config, "scheme")
	if scheme == "" {
		scheme = "https"
	}

	port := providerbuiltin.String(config, "port")
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
	tokenID, err := providerbuiltin.ResolveConfigValue(config, "token_id")
	if err != nil {
		return nil, err
	}
	tokenSecret, err := providerbuiltin.ResolveConfigValue(config, "token_secret")
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

	username := providerbuiltin.String(config, "username")
	if username == "" {
		username = providerbuiltin.String(config, "user")
	}
	password, err := providerbuiltin.ResolveConfigValue(config, "password")
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
		return nil, fmt.Errorf("proxmox auth failed: %s", strings.TrimSpace(string(body)))
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
	u.Path = "/api2/json/" + strings.TrimPrefix(path, "/")

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
		return nil, fmt.Errorf("proxmox api request failed: %s", strings.TrimSpace(string(body)))
	}
	return body, nil
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
