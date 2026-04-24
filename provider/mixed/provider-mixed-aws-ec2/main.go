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
	case providerapi.MethodPluginDescribe:
		_ = providerbuiltin.WriteResponse(req, providerapi.PluginDescribeResult{
			Name:            "provider-mixed-aws-ec2",
			Capabilities:    []string{"inventory", "connector"},
			ConnectorNames:  []string{"aws-ssm", "aws-eice"},
			Methods:         []string{providerapi.MethodPluginDescribe, providerapi.MethodHealthCheck, providerapi.MethodInventoryList, providerapi.MethodConnectorDescribe, providerapi.MethodConnectorPrepare},
			ProtocolVersion: providerapi.Version,
		}, nil)
	case providerapi.MethodInventoryList:
		var params providerapi.InventoryListParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}

		servers, err := listAWS(params.Config)
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
		result, err := awsHealthCheck(params.Config)
		if err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "health_check_failed", err.Error())
			os.Exit(1)
		}
		_ = providerbuiltin.WriteResponse(req, result, nil)
	case providerapi.MethodConnectorDescribe:
		var params providerapi.ConnectorDescribeParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := awsConnectorDescribe(params)
		if err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "connector_describe_failed", err.Error())
			os.Exit(1)
		}
		_ = providerbuiltin.WriteResponse(req, result, nil)
	case providerapi.MethodConnectorPrepare, providerapi.MethodTransportPrep:
		var params providerapi.ConnectorPrepareParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := awsConnectorPrepare(params)
		if err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "connector_prepare_failed", err.Error())
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

func listAWS(config map[string]interface{}) ([]providerapi.InventoryServer, error) {
	regions := providerbuiltin.StringSlice(config, "regions")
	if len(regions) == 0 {
		regions = []string{providerbuiltin.String(config, "region")}
	}
	if len(regions) == 0 || regions[0] == "" {
		regions = []string{"us-east-1"}
	}

	includeTags := providerbuiltin.StringSlice(config, "include_tags")
	addrStrategy := awsAddrStrategy(config)
	nameTemplate := providerbuiltin.String(config, "server_name_template")
	if nameTemplate == "" {
		nameTemplate = "aws:${tags.Name}"
	}
	noteTemplate := providerbuiltin.String(config, "note_template")

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
	if profile := providerbuiltin.String(raw, "profile"); profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}
	if v := providerbuiltin.ExpandPaths(providerbuiltin.StringSlice(raw, "shared_config_files")); len(v) > 0 {
		opts = append(opts, awsconfig.WithSharedConfigFiles(v))
	}
	if v := providerbuiltin.ExpandPaths(providerbuiltin.StringSlice(raw, "shared_credentials_files")); len(v) > 0 {
		opts = append(opts, awsconfig.WithSharedCredentialsFiles(v))
	}
	return awsconfig.LoadDefaultConfig(ctx, opts...)
}

func awsHealthCheck(config map[string]interface{}) (providerapi.HealthCheckResult, error) {
	regions := providerbuiltin.StringSlice(config, "regions")
	if len(regions) == 0 {
		regions = []string{providerbuiltin.String(config, "region")}
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
	if err := awsSSMHealthCheck(ctx, config, regions[0]); err != nil {
		return providerapi.HealthCheckResult{}, err
	}

	return providerapi.HealthCheckResult{
		OK:      true,
		Message: "aws mixed provider can access EC2 and SSM",
	}, nil
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
