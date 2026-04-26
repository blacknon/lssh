package eiceconnector

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/blacknon/lssh/providerapi"
	"github.com/blacknon/lssh/providerutil/tunnelcmd"
)

type Config struct {
	InstanceID             string
	Region                 string
	Profile                string
	EndpointID             string
	EndpointDNSName        string
	PrivateIPAddress       string
	RemotePort             string
	SharedConfigFiles      []string
	SharedCredentialsFiles []string
}

func ConfigFromPlan(plan providerapi.ConnectorPlan) (Config, error) {
	cfg := Config{
		InstanceID:             detailString(plan.Details, "instance_id"),
		Region:                 detailString(plan.Details, "region"),
		Profile:                detailString(plan.Details, "profile"),
		EndpointID:             detailString(plan.Details, "instance_connect_endpoint_id"),
		EndpointDNSName:        detailString(plan.Details, "instance_connect_endpoint_dns_name"),
		PrivateIPAddress:       detailString(plan.Details, "private_ip_address"),
		RemotePort:             detailString(plan.Details, "target_port"),
		SharedConfigFiles:      detailStringSlice(plan.Details, "shared_config_files"),
		SharedCredentialsFiles: detailStringSlice(plan.Details, "shared_credentials_files"),
	}
	if cfg.InstanceID == "" || cfg.Region == "" || cfg.RemotePort == "" {
		return Config{}, fmt.Errorf("aws eice plan is missing instance_id, region, or target_port")
	}
	return cfg, nil
}

func DialTarget(ctx context.Context, cfg Config) (net.Conn, error) {
	localPort, err := tunnelcmd.PickFreePort()
	if err != nil {
		return nil, err
	}

	args := []string{
		"ec2-instance-connect", "open-tunnel",
		"--instance-id", cfg.InstanceID,
		"--region", cfg.Region,
		"--remote-port", cfg.RemotePort,
		"--local-port", fmt.Sprint(localPort),
	}
	if cfg.Profile != "" {
		args = append(args, "--profile", cfg.Profile)
	}
	if cfg.EndpointID != "" {
		args = append(args, "--instance-connect-endpoint-id", cfg.EndpointID)
	}
	if cfg.EndpointDNSName != "" {
		args = append(args, "--instance-connect-endpoint-dns-name", cfg.EndpointDNSName)
	}
	if cfg.PrivateIPAddress != "" {
		args = append(args, "--private-ip-address", cfg.PrivateIPAddress)
	}

	conn, err := tunnelcmd.StartAndDial(ctx, append([]string{"aws"}, args...), commandEnv(cfg.SharedConfigFiles, cfg.SharedCredentialsFiles), "127.0.0.1", localPort, 20*time.Second)
	if err != nil {
		return nil, wrapAWSEICETunnelError(err)
	}
	return conn, nil
}

func commandEnv(sharedConfigFiles, sharedCredentialsFiles []string) map[string]string {
	env := map[string]string{}
	if len(sharedConfigFiles) > 0 && strings.TrimSpace(sharedConfigFiles[0]) != "" {
		env["AWS_CONFIG_FILE"] = sharedConfigFiles[0]
	}
	if len(sharedCredentialsFiles) > 0 && strings.TrimSpace(sharedCredentialsFiles[0]) != "" {
		env["AWS_SHARED_CREDENTIALS_FILE"] = sharedCredentialsFiles[0]
	}
	return env
}

func detailString(details map[string]interface{}, key string) string {
	if details == nil {
		return ""
	}
	if value, ok := details[key]; ok && value != nil {
		return fmt.Sprint(value)
	}
	return ""
}

func detailStringSlice(details map[string]interface{}, key string) []string {
	if details == nil {
		return nil
	}
	raw, ok := details[key]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if item == nil {
				continue
			}
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return []string{fmt.Sprint(typed)}
	}
}

func FirstConfigFile(sharedConfigFiles []string) string {
	if len(sharedConfigFiles) == 0 {
		return ""
	}
	return sharedConfigFiles[0]
}

func FirstCredentialsFile(sharedCredentialsFiles []string) string {
	if len(sharedCredentialsFiles) == 0 {
		return ""
	}
	return sharedCredentialsFiles[0]
}

func SSHProxyCommand(cfg Config) string {
	args := []string{
		"aws", "ec2-instance-connect", "open-tunnel",
		"--instance-id", cfg.InstanceID,
		"--region", cfg.Region,
		"--remote-port", cfg.RemotePort,
	}
	if cfg.Profile != "" {
		args = append(args, "--profile", cfg.Profile)
	}
	if cfg.EndpointID != "" {
		args = append(args, "--instance-connect-endpoint-id", cfg.EndpointID)
	}
	if cfg.EndpointDNSName != "" {
		args = append(args, "--instance-connect-endpoint-dns-name", cfg.EndpointDNSName)
	}
	if cfg.PrivateIPAddress != "" {
		args = append(args, "--private-ip-address", cfg.PrivateIPAddress)
	}
	return strings.Join(args, " ")
}

func CommandEnv(cfg Config) map[string]string {
	return commandEnv(cfg.SharedConfigFiles, cfg.SharedCredentialsFiles)
}

func ExpandForCommand(cfg Config) Config {
	if cfg.RemotePort == "" {
		cfg.RemotePort = "22"
	}
	if len(cfg.SharedConfigFiles) > 0 {
		cfg.SharedConfigFiles[0] = expandPath(cfg.SharedConfigFiles[0])
	}
	if len(cfg.SharedCredentialsFiles) > 0 {
		cfg.SharedCredentialsFiles[0] = expandPath(cfg.SharedCredentialsFiles[0])
	}
	return cfg
}

func expandPath(path string) string {
	if strings.TrimSpace(path) == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return home + path[1:]
	}
	return path
}

func wrapAWSEICETunnelError(err error) error {
	if err == nil {
		return nil
	}

	hint := "hint: verify the EC2 Instance Connect Endpoint ID/DNS, target private IP, endpoint security groups, and IAM permission for ec2-instance-connect:OpenTunnel"
	return fmt.Errorf("aws eice tunnel failed: %w; %s", err, hint)
}
