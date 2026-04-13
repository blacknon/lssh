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
	case providerapi.MethodInventoryList:
		var params providerapi.InventoryListParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteError(err.Error())
			os.Exit(1)
		}

		servers, err := listAWS(params.Config)
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

func listAWS(config map[string]interface{}) ([]providerapi.InventoryServer, error) {
	regions := providerbuiltin.StringSlice(config, "regions")
	if len(regions) == 0 {
		regions = []string{providerbuiltin.String(config, "region")}
	}
	if len(regions) == 0 || regions[0] == "" {
		regions = []string{"us-east-1"}
	}

	includeTags := providerbuiltin.StringSlice(config, "include_tags")
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

					name := renderTemplate(nameTemplate, instanceID, privateIP, publicIP, region, tags)
					if name == "" {
						name = "aws:" + instanceID
					}

					cfgMap := map[string]interface{}{
						"addr": privateIP,
						"note": renderTemplate(noteTemplate, instanceID, privateIP, publicIP, region, tags),
					}
					if cfgMap["addr"] == "" {
						cfgMap["addr"] = publicIP
					}
					for _, key := range includeTags {
						if value := tags[key]; value != "" {
							cfgMap["tag_"+strings.ToLower(key)] = value
						}
					}

					meta := map[string]string{
						"provider":    "aws",
						"plugin":      "provider-inventory-aws-ec2",
						"instance_id": instanceID,
						"private_ip":  privateIP,
						"public_ip":   publicIP,
						"region":      region,
						"zone":        zone,
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

func loadAWSConfig(ctx context.Context, raw map[string]interface{}, region string) (cfg aws.Config, err error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if profile := providerbuiltin.String(raw, "profile"); profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}
	if v := providerbuiltin.StringSlice(raw, "shared_config_files"); len(v) > 0 {
		opts = append(opts, awsconfig.WithSharedConfigFiles(v))
	}
	if v := providerbuiltin.StringSlice(raw, "shared_credentials_files"); len(v) > 0 {
		opts = append(opts, awsconfig.WithSharedCredentialsFiles(v))
	}
	return awsconfig.LoadDefaultConfig(ctx, opts...)
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
