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
	case providerapi.MethodPluginDescribe:
		_ = providerbuiltin.WriteResponse(req, providerapi.PluginDescribeResult{
			Name:            "provider-inventory-gcp-compute",
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

		servers, err := listGCP(params.Config)
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
		result, err := gcpHealthCheck(params.Config)
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
	addrStrategy := gcpAddrStrategy(config)
	zone := providerbuiltin.String(config, "zone")

	if zone != "" {
		return listGCPZone(ctx, service, project, zone, nameTemplate, noteTemplate, addrStrategy)
	}
	return listGCPAggregated(ctx, service, project, nameTemplate, noteTemplate, addrStrategy)
}

func listGCPZone(ctx context.Context, service *compute.Service, project, zone, nameTemplate, noteTemplate, addrStrategy string) ([]providerapi.InventoryServer, error) {
	call := service.Instances.List(project, zone)
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return convertGCPInstances(resp.Items, nameTemplate, noteTemplate, addrStrategy), nil
}

func listGCPAggregated(ctx context.Context, service *compute.Service, project, nameTemplate, noteTemplate, addrStrategy string) ([]providerapi.InventoryServer, error) {
	call := service.Instances.AggregatedList(project)
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	out := []providerapi.InventoryServer{}
	for _, scoped := range resp.Items {
		out = append(out, convertGCPInstances(scoped.Instances, nameTemplate, noteTemplate, addrStrategy)...)
	}
	return out, nil
}

func convertGCPInstances(instances []*compute.Instance, nameTemplate, noteTemplate, addrStrategy string) []providerapi.InventoryServer {
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

		addr := gcpSelectAddress(privateIP, publicIP, addrStrategy)
		cfg := map[string]interface{}{
			"addr": addr,
			"note": renderGCPTemplate(noteTemplate, instance.Name, fmt.Sprint(instance.Id), privateIP, publicIP, instance.Zone, instance.Labels),
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
	if credentialsFile := gcpCredentialsFile(config); credentialsFile != "" {
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

func gcpAddrStrategy(config map[string]interface{}) string {
	switch strings.TrimSpace(strings.ToLower(providerbuiltin.String(config, "addr_strategy"))) {
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

func gcpSelectAddress(privateIP, publicIP, strategy string) string {
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

func gcpCredentialsFile(config map[string]interface{}) string {
	credentialsFile := providerbuiltin.String(config, "credentials_file")
	if credentialsFile == "" {
		return ""
	}
	return providerbuiltin.ExpandPath(credentialsFile)
}

func gcpHealthCheck(config map[string]interface{}) (providerapi.HealthCheckResult, error) {
	project := providerbuiltin.String(config, "project")
	if project == "" {
		return providerapi.HealthCheckResult{}, fmt.Errorf("project is required")
	}

	ctx := context.Background()
	service, err := compute.NewService(ctx, gcpOptions(config)...)
	if err != nil {
		return providerapi.HealthCheckResult{}, err
	}

	call := service.Zones.List(project).MaxResults(1)
	if _, err := call.Context(ctx).Do(); err != nil {
		return providerapi.HealthCheckResult{}, err
	}

	return providerapi.HealthCheckResult{
		OK:      true,
		Message: "gcp inventory provider can access Compute Engine",
	}, nil
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
