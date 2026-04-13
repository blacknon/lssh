package main

import (
	"encoding/json"
	"fmt"
	"os"
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
	data, err := providerbuiltin.Run("pvesh", "get", "/cluster/resources", "--type", "vm", "--output-format", "json")
	if err != nil {
		return nil, err
	}

	var resources []struct {
		Name   string `json:"name"`
		Node   string `json:"node"`
		Status string `json:"status"`
		VMID   int    `json:"vmid"`
	}
	if err := json.Unmarshal(data, &resources); err != nil {
		return nil, err
	}

	nameTemplate := providerbuiltin.String(config, "server_name_template")
	if nameTemplate == "" {
		nameTemplate = "pve:${node}:${name}"
	}
	noteTemplate := providerbuiltin.String(config, "note_template")
	nodeAddrPrefix := providerbuiltin.String(config, "node_addr_prefix")

	out := make([]providerapi.InventoryServer, 0, len(resources))
	for _, resource := range resources {
		if resource.Status != "running" {
			continue
		}
		addr := ""
		if nodeAddrPrefix != "" {
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
			},
		})
	}

	return out, nil
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
