package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/blacknon/lssh/internal/providerapi"
	"github.com/blacknon/lssh/internal/providerbuiltin"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
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

		servers, err := listGCP(params.Config)
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

func listGCP(config map[string]interface{}) ([]providerapi.InventoryServer, error) {
	project := providerbuiltin.String(config, "project")
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}

	ctx := context.Background()
	service, err := compute.NewService(ctx, gcpOptions(config)...)
	if err != nil {
		return nil, err
	}

	nameTemplate := providerbuiltin.String(config, "server_name_template")
	if nameTemplate == "" {
		nameTemplate = "gcp:${name}"
	}
	noteTemplate := providerbuiltin.String(config, "note_template")
	zone := providerbuiltin.String(config, "zone")

	if zone != "" {
		return listGCPZone(ctx, service, project, zone, nameTemplate, noteTemplate)
	}
	return listGCPAggregated(ctx, service, project, nameTemplate, noteTemplate)
}

func listGCPZone(ctx context.Context, service *compute.Service, project, zone, nameTemplate, noteTemplate string) ([]providerapi.InventoryServer, error) {
	call := service.Instances.List(project, zone)
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return convertGCPInstances(resp.Items, nameTemplate, noteTemplate), nil
}

func listGCPAggregated(ctx context.Context, service *compute.Service, project, nameTemplate, noteTemplate string) ([]providerapi.InventoryServer, error) {
	call := service.Instances.AggregatedList(project)
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	out := []providerapi.InventoryServer{}
	for _, scoped := range resp.Items {
		out = append(out, convertGCPInstances(scoped.Instances, nameTemplate, noteTemplate)...)
	}
	return out, nil
}

func convertGCPInstances(instances []*compute.Instance, nameTemplate, noteTemplate string) []providerapi.InventoryServer {
	out := make([]providerapi.InventoryServer, 0, len(instances))
	for _, instance := range instances {
		if instance == nil || instance.Status != "RUNNING" {
			continue
		}

		privateIP := ""
		publicIP := ""
		if len(instance.NetworkInterfaces) > 0 {
			privateIP = instance.NetworkInterfaces[0].NetworkIP
			if len(instance.NetworkInterfaces[0].AccessConfigs) > 0 {
				publicIP = instance.NetworkInterfaces[0].AccessConfigs[0].NatIP
			}
		}

		cfg := map[string]interface{}{
			"addr": privateIP,
			"note": renderGCPTemplate(noteTemplate, instance.Name, fmt.Sprint(instance.Id), privateIP, publicIP, instance.Zone, instance.Labels),
		}
		if cfg["addr"] == "" {
			cfg["addr"] = publicIP
		}

		meta := map[string]string{
			"provider":   "gcp",
			"plugin":     "provider-inventory-gcp-compute",
			"id":         fmt.Sprint(instance.Id),
			"name":       instance.Name,
			"private_ip": privateIP,
			"public_ip":  publicIP,
			"zone":       basename(instance.Zone),
		}
		for key, value := range instance.Labels {
			meta["label."+key] = value
		}

		out = append(out, providerapi.InventoryServer{
			Name:   renderGCPTemplate(nameTemplate, instance.Name, fmt.Sprint(instance.Id), privateIP, publicIP, instance.Zone, instance.Labels),
			Config: cfg,
			Meta:   meta,
		})
	}

	return out
}

func gcpOptions(config map[string]interface{}) []option.ClientOption {
	opts := []option.ClientOption{}
	if credentialsFile := providerbuiltin.String(config, "credentials_file"); credentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}
	if endpoint := providerbuiltin.String(config, "endpoint"); endpoint != "" {
		opts = append(opts, option.WithEndpoint(endpoint))
	}
	if scopes := providerbuiltin.StringSlice(config, "scopes"); len(scopes) > 0 {
		opts = append(opts, option.WithScopes(scopes...))
	}
	return opts
}

func basename(v string) string {
	if i := strings.LastIndex(v, "/"); i >= 0 {
		return v[i+1:]
	}
	return v
}

func renderGCPTemplate(template, name, id, privateIP, publicIP, zone string, labels map[string]string) string {
	if template == "" {
		return ""
	}
	zone = basename(zone)
	replacerArgs := []string{
		"${name}", name,
		"${id}", id,
		"${private_ip}", privateIP,
		"${public_ip}", publicIP,
		"${zone}", zone,
	}
	for key, value := range labels {
		replacerArgs = append(replacerArgs, "${labels."+key+"}", value)
	}
	return strings.NewReplacer(replacerArgs...).Replace(template)
}
