package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
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
			Name:           "provider-mixed-aws-ec2",
			Capabilities:   []string{"inventory", "connector"},
			ConnectorNames: []string{"aws-ssm", "aws-eice"},
			Methods:        []string{providerapi.MethodPluginDescribe, providerapi.MethodHealthCheck, providerapi.MethodInventoryList, providerapi.MethodConnectorDescribe, providerapi.MethodConnectorPrepare, providerapi.MethodConnectorShell, providerapi.MethodConnectorExec, providerapi.MethodConnectorDial},
			ReservedKeys: []string{
				"regions", "region", "profile",
				"shared_config_files", "shared_credentials_files",
				"include_tags", "server_name_template", "note_template", "addr_strategy",
				"ssm_shell_runtime", "ssm_port_forward_runtime",
				"eice_runtime", "instance_connect_endpoint_id", "instance_connect_endpoint_dns_name", "private_ip_address",
				"ssm_require_online", "ssm_shell_document", "ssm_interactive_command_document", "ssm_port_forward_document",
			},
			ProtocolVersion: providerapi.Version,
		}, nil)
	case providerapi.MethodInventoryList:
		var params providerapi.InventoryListParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}

		servers, err := listAWS(params.Config)
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
		result, err := awsHealthCheck(params.Config)
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
		result, err := awsConnectorDescribe(params)
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
		result, err := awsConnectorPrepare(params)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "connector_prepare_failed", err.Error())
			os.Exit(1)
		}
		_ = providerapi.WriteResponse(req, result, nil)
	case providerapi.MethodConnectorShell:
		var params providerapi.ConnectorRuntimeParams
		if err := decodeParams(req.Params, &params); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := awsConnectorRunShell(params); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case providerapi.MethodConnectorExec:
		var params providerapi.ConnectorRuntimeParams
		if err := decodeParams(req.Params, &params); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		result, err := awsConnectorRunExec(params)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := providerapi.WriteRuntimeResult(result); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case providerapi.MethodConnectorDial:
		var params providerapi.ConnectorRuntimeParams
		if err := decodeParams(req.Params, &params); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := awsConnectorRunDial(params); err != nil {
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

func listAWS(config map[string]interface{}) ([]providerapi.InventoryServer, error) {
	regions := providerapi.StringSlice(config, "regions")
	if len(regions) == 0 {
		regions = []string{providerapi.String(config, "region")}
	}
	if len(regions) == 0 || regions[0] == "" {
		regions = []string{"us-east-1"}
	}

	includeTags := providerapi.StringSlice(config, "include_tags")
	addrStrategy := awsAddrStrategy(config)
	nameTemplate := providerapi.String(config, "server_name_template")
	if nameTemplate == "" {
		nameTemplate = "aws:${tags.Name}"
	}
	noteTemplate := providerapi.String(config, "note_template")

	ctx := context.Background()
	var out []providerapi.InventoryServer
	for _, region := range regions {
		cfg, err := loadAWSConfig(ctx, config, region)
		if err != nil {
			return nil, err
		}

		client := ec2.NewFromConfig(cfg)
		paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, err
			}

			for _, reservation := range page.Reservations {
				for _, instance := range reservation.Instances {
					if instance.State == nil || instance.State.Name != ec2types.InstanceStateNameRunning {
						continue
					}

					tags := awsTags(instance.Tags)
					privateIP := awsString(instance.PrivateIpAddress)
					publicIP := awsString(instance.PublicIpAddress)
					instanceID := awsString(instance.InstanceId)
					zone := ""
					if instance.Placement != nil {
						zone = awsString(instance.Placement.AvailabilityZone)
					}
					platform := "linux"
					if instance.Platform == ec2types.PlatformValuesWindows {
						platform = "windows"
					}

					name := renderTemplate(nameTemplate, instanceID, privateIP, publicIP, region, tags)
					if name == "" {
						name = "aws:" + instanceID
					}

					addr := awsSelectAddress(privateIP, publicIP, addrStrategy)
					cfgMap := map[string]interface{}{
						"addr": addr,
						"note": renderTemplate(noteTemplate, instanceID, privateIP, publicIP, region, tags),
					}
					for _, key := range includeTags {
						if value := tags[key]; value != "" {
							cfgMap["tag_"+strings.ToLower(key)] = value
						}
					}

					meta := map[string]string{
						"provider":    "aws",
						"plugin":      "provider-mixed-aws-ec2",
						"instance_id": instanceID,
						"private_ip":  privateIP,
						"public_ip":   publicIP,
						"region":      region,
						"zone":        zone,
						"platform":    platform,
					}
					for key, value := range tags {
						meta["tag."+key] = value
					}

					out = append(out, providerapi.InventoryServer{Name: name, Config: cfgMap, Meta: meta})
				}
			}
		}
	}

	return out, nil
}

func awsAddrStrategy(config map[string]interface{}) string {
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

func awsSelectAddress(privateIP, publicIP, strategy string) string {
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

func loadAWSConfig(ctx context.Context, raw map[string]interface{}, region string) (cfg aws.Config, err error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if profile := providerapi.String(raw, "profile"); profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}
	if v := providerapi.ExpandPaths(providerapi.StringSlice(raw, "shared_config_files")); len(v) > 0 {
		opts = append(opts, awsconfig.WithSharedConfigFiles(v))
	}
	if v := providerapi.ExpandPaths(providerapi.StringSlice(raw, "shared_credentials_files")); len(v) > 0 {
		opts = append(opts, awsconfig.WithSharedCredentialsFiles(v))
	}
	return awsconfig.LoadDefaultConfig(ctx, opts...)
}

func awsHealthCheck(config map[string]interface{}) (providerapi.HealthCheckResult, error) {
	regions := providerapi.StringSlice(config, "regions")
	if len(regions) == 0 {
		regions = []string{providerapi.String(config, "region")}
	}
	if len(regions) == 0 || regions[0] == "" {
		regions = []string{"us-east-1"}
	}

	ctx := context.Background()
	cfg, err := loadAWSConfig(ctx, config, regions[0])
	if err != nil {
		return providerapi.HealthCheckResult{}, err
	}

	client := ec2.NewFromConfig(cfg)
	if _, err := client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{}); err != nil {
		return providerapi.HealthCheckResult{}, err
	}

	checkEC2, checkSSM := awsHealthCheckScopes(config)
	if checkSSM {
		if err := awsSSMHealthCheck(ctx, config, regions[0]); err != nil {
			return providerapi.HealthCheckResult{}, err
		}
	}

	return providerapi.HealthCheckResult{
		OK:      true,
		Message: awsHealthCheckMessage(checkEC2, checkSSM),
	}, nil
}

func awsHealthCheckScopes(config map[string]interface{}) (checkEC2, checkSSM bool) {
	checkEC2 = true

	capabilities := normalizeLowerStrings(providerapi.StringSlice(config, "capabilities"))
	connectorNames := normalizeLowerStrings(providerapi.StringSlice(config, "connector_names"))

	if len(capabilities) > 0 {
		checkSSM = containsString(capabilities, "connector")
		return checkEC2, checkSSM
	}

	if len(connectorNames) > 0 {
		for _, name := range connectorNames {
			switch name {
			case "aws-ssm":
				checkSSM = true
			}
		}
		return checkEC2, checkSSM
	}

	// Backward-compatible default: mixed provider health still validates both
	// EC2 inventory and the SSM connector unless the config scopes it down.
	checkSSM = true
	return checkEC2, checkSSM
}

func awsHealthCheckMessage(checkEC2, checkSSM bool) string {
	switch {
	case checkEC2 && checkSSM:
		return "aws mixed provider can access EC2 and SSM"
	case checkEC2:
		return "aws mixed provider can access EC2"
	case checkSSM:
		return "aws mixed provider can access SSM"
	default:
		return "aws mixed provider health check completed"
	}
}

func normalizeLowerStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func awsString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func awsTags(tags []ec2types.Tag) map[string]string {
	out := map[string]string{}
	for _, tag := range tags {
		key := awsString(tag.Key)
		if key == "" {
			continue
		}
		out[key] = awsString(tag.Value)
	}
	return out
}

func renderTemplate(template, instanceID, privateIP, publicIP, region string, tags map[string]string) string {
	if template == "" {
		return ""
	}
	replacerArgs := []string{
		"${instance_id}", instanceID,
		"${private_ip}", privateIP,
		"${public_ip}", publicIP,
		"${region}", region,
	}
	for key, value := range tags {
		replacerArgs = append(replacerArgs, "${tags."+key+"}", value)
	}
	return strings.NewReplacer(replacerArgs...).Replace(template)
}
