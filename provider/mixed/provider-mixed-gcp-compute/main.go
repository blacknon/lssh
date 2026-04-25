package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/blacknon/lssh/providerapi"
	"github.com/kballard/go-shellquote"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
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
			Name:            "provider-mixed-gcp-compute",
			Capabilities:    []string{"inventory", "connector"},
			ConnectorNames:  []string{"gcp-iap"},
			Methods:         []string{providerapi.MethodPluginDescribe, providerapi.MethodHealthCheck, providerapi.MethodInventoryList, providerapi.MethodConnectorDescribe, providerapi.MethodConnectorPrepare},
			ProtocolVersion: providerapi.Version,
		}, nil)
	case providerapi.MethodInventoryList:
		var params providerapi.InventoryListParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}

		servers, err := listGCP(params.Config)
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
		result, err := gcpHealthCheck(params.Config)
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
		result, err := gcpConnectorDescribe(params)
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
		result, err := gcpConnectorPrepare(params)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "connector_prepare_failed", err.Error())
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

func listGCP(config map[string]interface{}) ([]providerapi.InventoryServer, error) {
	project := providerapi.String(config, "project")
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}

	ctx := context.Background()
	service, err := compute.NewService(ctx, gcpOptions(config)...)
	if err != nil {
		return nil, err
	}

	nameTemplate := providerapi.String(config, "server_name_template")
	if nameTemplate == "" {
		nameTemplate = "gcp:${name}"
	}
	noteTemplate := providerapi.String(config, "note_template")
	addrStrategy := gcpAddrStrategy(config)
	zone := providerapi.String(config, "zone")

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
	return convertGCPInstances(resp.Items, project, nameTemplate, noteTemplate, addrStrategy), nil
}

func listGCPAggregated(ctx context.Context, service *compute.Service, project, nameTemplate, noteTemplate, addrStrategy string) ([]providerapi.InventoryServer, error) {
	call := service.Instances.AggregatedList(project)
	resp, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	out := []providerapi.InventoryServer{}
	for _, scoped := range resp.Items {
		out = append(out, convertGCPInstances(scoped.Instances, project, nameTemplate, noteTemplate, addrStrategy)...)
	}
	return out, nil
}

func convertGCPInstances(instances []*compute.Instance, project, nameTemplate, noteTemplate, addrStrategy string) []providerapi.InventoryServer {
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
			"plugin":     "provider-mixed-gcp-compute",
			"id":         fmt.Sprint(instance.Id),
			"name":       instance.Name,
			"project":    project,
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
	if endpoint := providerapi.String(config, "endpoint"); endpoint != "" {
		opts = append(opts, option.WithEndpoint(endpoint))
	}
	if scopes := providerapi.StringSlice(config, "scopes"); len(scopes) > 0 {
		opts = append(opts, option.WithScopes(scopes...))
	}
	return opts
}

func gcpAddrStrategy(config map[string]interface{}) string {
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
	credentialsFile := providerapi.String(config, "credentials_file")
	if credentialsFile == "" {
		return ""
	}
	return providerapi.ExpandPath(credentialsFile)
}

func gcpHealthCheck(config map[string]interface{}) (providerapi.HealthCheckResult, error) {
	project := providerapi.String(config, "project")
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

func gcpConnectorDescribe(params providerapi.ConnectorDescribeParams) (providerapi.ConnectorDescribeResult, error) {
	cfg, err := gcpConnectorConfig(params.Config, params.Target)
	if err != nil {
		return providerapi.ConnectorDescribeResult{}, err
	}

	commandRuntime := gcpConnectorRuntime(params.Config, params.Target.Config) == "command"
	sdkRuntime := !commandRuntime
	sdkReason := ""
	if !sdkRuntime {
		sdkReason = "gcp-iap sdk runtime is disabled by configuration"
	}

	mountSupported := sdkRuntime && (runtime.GOOS == "linux" || runtime.GOOS == "darwin")
	mountReason := sdkReason
	if sdkRuntime && !mountSupported {
		mountReason = "mount via gcp-iap sdk runtime currently supports linux and macos only"
	}

	return providerapi.ConnectorDescribeResult{
		Capabilities: map[string]providerapi.ConnectorCapability{
			"shell": {
				Supported: cfg.InstanceName != "",
				Reason:    unsupportedReason(cfg.InstanceName != "", "gcp-iap requires instance_name metadata"),
				Preferred: sdkRuntime,
			},
			"exec": {
				Supported: cfg.InstanceName != "",
				Reason:    unsupportedReason(cfg.InstanceName != "", "gcp-iap requires instance_name metadata"),
				Preferred: true,
			},
			"exec_pty": {
				Supported: sdkRuntime && cfg.InstanceName != "",
				Reason:    unsupportedReason(sdkRuntime && cfg.InstanceName != "", sdkReason),
			},
			"sftp_transport": {
				Supported: sdkRuntime && cfg.InstanceName != "",
				Reason:    unsupportedReason(sdkRuntime && cfg.InstanceName != "", sdkReason),
			},
			"upload": {
				Supported: sdkRuntime && cfg.InstanceName != "",
				Reason:    unsupportedReason(sdkRuntime && cfg.InstanceName != "", sdkReason),
				Requires:  []string{"sftp_transport"},
			},
			"download": {
				Supported: sdkRuntime && cfg.InstanceName != "",
				Reason:    unsupportedReason(sdkRuntime && cfg.InstanceName != "", sdkReason),
				Requires:  []string{"sftp_transport"},
			},
			"mount": {
				Supported: mountSupported && cfg.InstanceName != "",
				Reason:    unsupportedReason(mountSupported && cfg.InstanceName != "", mountReason),
				Requires:  []string{"sftp_transport"},
			},
			"port_forward_local": {
				Supported: cfg.InstanceName != "",
				Reason:    unsupportedReason(cfg.InstanceName != "", "gcp-iap requires instance_name metadata"),
			},
			"tcp_dial_transport": {
				Supported: sdkRuntime && cfg.InstanceName != "",
				Reason:    unsupportedReason(sdkRuntime && cfg.InstanceName != "", sdkReason),
			},
			"port_forward_remote": {
				Supported: false,
				Reason:    "gcp-iap does not support remote port forwarding in the first connector wave",
			},
			"agent_forward": {
				Supported: false,
				Reason:    "gcp-iap command and sdk runtimes do not provide agent forwarding in the first connector wave",
			},
		},
	}, nil
}

func gcpConnectorPrepare(params providerapi.ConnectorPrepareParams) (providerapi.ConnectorPrepareResult, error) {
	cfg, err := gcpConnectorConfig(params.Config, params.Target)
	if err != nil {
		return providerapi.ConnectorPrepareResult{}, err
	}

	if gcpConnectorRuntime(params.Config, params.Target.Config) == "command" {
		return gcpConnectorPrepareCommand(params, cfg)
	}
	return gcpConnectorPrepareSDK(params, cfg), nil
}

type gcpIAPConfig struct {
	Project      string
	Zone         string
	InstanceName string
	Interface    string
	User         string
	KeyFile      string
	Port         string
	Credentials  string
	Scopes       []string
}

func gcpConnectorConfig(config map[string]interface{}, target providerapi.ConnectorTarget) (gcpIAPConfig, error) {
	cfg := gcpIAPConfig{
		Project:      firstNonEmpty(providerapi.String(target.Config, "project"), providerapi.String(config, "project")),
		Zone:         firstNonEmpty(target.Meta["zone"], providerapi.String(target.Config, "zone"), providerapi.String(config, "zone")),
		InstanceName: firstNonEmpty(target.Meta["name"], providerapi.String(target.Config, "instance_name")),
		Interface:    firstNonEmpty(providerapi.String(target.Config, "interface"), providerapi.String(config, "interface"), "nic0"),
		User:         providerapi.String(target.Config, "user"),
		KeyFile:      providerapi.ExpandPath(providerapi.String(target.Config, "key")),
		Port:         firstNonEmpty(providerapi.String(target.Config, "port"), "22"),
		Credentials:  providerapi.ExpandPath(firstNonEmpty(providerapi.String(target.Config, "credentials_file"), providerapi.String(config, "credentials_file"))),
		Scopes:       firstNonEmptyStringSlice(providerapi.StringSlice(target.Config, "scopes"), providerapi.StringSlice(config, "scopes")),
	}
	if cfg.Project == "" {
		return cfg, fmt.Errorf("project is required for gcp-iap connector")
	}
	if cfg.Zone == "" {
		return cfg, fmt.Errorf("zone is required for gcp-iap connector")
	}
	if cfg.InstanceName == "" {
		return cfg, fmt.Errorf("instance_name metadata is required for gcp-iap connector")
	}
	return cfg, nil
}

func gcpConnectorPrepareSDK(params providerapi.ConnectorPrepareParams, cfg gcpIAPConfig) providerapi.ConnectorPrepareResult {
	targetPort := firstNonEmpty(gcpOptionString(params.Operation.Options, "target_port"), cfg.Port)
	if params.Operation.Name == "port_forward_local" || params.Operation.Name == "tcp_dial_transport" {
		targetPort = firstNonEmpty(gcpOptionString(params.Operation.Options, "target_port"), cfg.Port)
	}
	details := map[string]interface{}{
		"connector":        "gcp-iap",
		"ssh_runtime":      "sdk",
		"transport":        "ssh_transport",
		"operation":        params.Operation.Name,
		"project":          cfg.Project,
		"zone":             cfg.Zone,
		"instance_name":    cfg.InstanceName,
		"interface":        cfg.Interface,
		"target_port":      targetPort,
		"credentials_file": cfg.Credentials,
	}
	if len(cfg.Scopes) > 0 {
		scopeValues := make([]interface{}, 0, len(cfg.Scopes))
		for _, scope := range cfg.Scopes {
			scopeValues = append(scopeValues, scope)
		}
		details["scopes"] = scopeValues
	}
	return providerapi.ConnectorPrepareResult{
		Supported: true,
		Plan: providerapi.ConnectorPlan{
			Kind:    "provider-managed",
			Details: details,
		},
	}
}

func gcpConnectorPrepareCommand(params providerapi.ConnectorPrepareParams, cfg gcpIAPConfig) (providerapi.ConnectorPrepareResult, error) {
	baseArgs := []string{"compute", "ssh", gcpDestination(cfg), "--project", cfg.Project, "--zone", cfg.Zone, "--tunnel-through-iap"}
	if cfg.KeyFile != "" {
		baseArgs = append(baseArgs, "--ssh-key-file", cfg.KeyFile)
	}
	if cfg.Port != "" && cfg.Port != "22" {
		baseArgs = append(baseArgs, "--ssh-flag", "-p", "--ssh-flag", cfg.Port)
	}

	switch params.Operation.Name {
	case "shell":
		return providerapi.ConnectorPrepareResult{
			Supported: true,
			Plan: providerapi.ConnectorPlan{
				Kind:    "command",
				Program: "gcloud",
				Args:    baseArgs,
				Details: map[string]interface{}{"connector": "gcp-iap", "operation": "shell", "runtime": "command"},
			},
		}, nil
	case "exec", "exec_pty":
		commandLine := strings.TrimSpace(gcpOptionString(params.Operation.Options, "command_line"))
		if commandLine == "" {
			commandLine = shellquote.Join(params.Operation.Command...)
		}
		if commandLine == "" {
			return providerapi.ConnectorPrepareResult{}, fmt.Errorf("gcp-iap command runtime requires a command")
		}
		args := append([]string{}, baseArgs...)
		args = append(args, "--command", commandLine)
		return providerapi.ConnectorPrepareResult{
			Supported: true,
			Plan: providerapi.ConnectorPlan{
				Kind:    "command",
				Program: "gcloud",
				Args:    args,
				Details: map[string]interface{}{"connector": "gcp-iap", "operation": params.Operation.Name, "runtime": "command"},
			},
		}, nil
	case "port_forward_local":
		listenHost := firstNonEmpty(gcpOptionString(params.Operation.Options, "listen_host"), "localhost")
		listenPort := gcpOptionString(params.Operation.Options, "listen_port")
		targetHost := gcpOptionString(params.Operation.Options, "target_host")
		targetPort := firstNonEmpty(gcpOptionString(params.Operation.Options, "target_port"), cfg.Port)
		if listenPort == "" || targetHost == "" || targetPort == "" {
			return providerapi.ConnectorPrepareResult{}, fmt.Errorf("gcp-iap local forward requires listen_port, target_host, and target_port")
		}
		args := append([]string{}, baseArgs...)
		args = append(args, "--", "-N", "-L", fmt.Sprintf("%s:%s:%s", net.JoinHostPort(listenHost, listenPort), targetHost, targetPort))
		return providerapi.ConnectorPrepareResult{
			Supported: true,
			Plan: providerapi.ConnectorPlan{
				Kind:    "command",
				Program: "gcloud",
				Args:    args,
				Details: map[string]interface{}{"connector": "gcp-iap", "operation": "port_forward_local", "runtime": "command"},
			},
		}, nil
	default:
		return providerapi.ConnectorPrepareResult{
			Supported: false,
			Plan: providerapi.ConnectorPlan{
				Kind: "command",
				Details: map[string]interface{}{
					"connector": "gcp-iap",
					"reason":    fmt.Sprintf("operation %q is not supported by the gcp-iap command runtime", params.Operation.Name),
				},
			},
		}, nil
	}
}

func gcpConnectorRuntime(config map[string]interface{}, targetConfig map[string]interface{}) string {
	switch strings.ToLower(strings.TrimSpace(firstNonEmpty(
		providerapi.String(targetConfig, "iap_runtime"),
		providerapi.String(config, "iap_runtime"),
	))) {
	case "", "sdk":
		return "sdk"
	case "command":
		return "command"
	default:
		return "sdk"
	}
}

func gcpDestination(cfg gcpIAPConfig) string {
	if cfg.User == "" {
		return cfg.InstanceName
	}
	return cfg.User + "@" + cfg.InstanceName
}

func gcpOptionString(raw map[string]interface{}, key string) string {
	if raw == nil {
		return ""
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func firstNonEmptyStringSlice(candidates ...[]string) []string {
	for _, candidate := range candidates {
		if len(candidate) > 0 {
			return candidate
		}
	}
	return nil
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
