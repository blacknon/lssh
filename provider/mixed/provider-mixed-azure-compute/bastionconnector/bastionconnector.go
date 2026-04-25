package bastionconnector

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/blacknon/lssh/providerapi"
	"github.com/blacknon/lssh/provider/connector/tunnelcmd"
)

type Config struct {
	BastionName      string
	ResourceGroup    string
	TargetResourceID string
	TargetPort       string
	SubscriptionID   string
}

func ConfigFromPlan(plan providerapi.ConnectorPlan) (Config, error) {
	cfg := Config{
		BastionName:      detailString(plan.Details, "bastion_name"),
		ResourceGroup:    detailString(plan.Details, "bastion_resource_group"),
		TargetResourceID: detailString(plan.Details, "target_resource_id"),
		TargetPort:       detailString(plan.Details, "target_port"),
		SubscriptionID:   detailString(plan.Details, "subscription_id"),
	}
	if cfg.BastionName == "" || cfg.ResourceGroup == "" || cfg.TargetResourceID == "" || cfg.TargetPort == "" {
		return Config{}, fmt.Errorf("azure bastion plan is missing bastion_name, bastion_resource_group, target_resource_id, or target_port")
	}
	return cfg, nil
}

func DialTarget(ctx context.Context, cfg Config) (net.Conn, error) {
	localPort, err := tunnelcmd.PickFreePort()
	if err != nil {
		return nil, err
	}

	command := []string{
		"az", "network", "bastion", "tunnel",
		"--name", cfg.BastionName,
		"--resource-group", cfg.ResourceGroup,
		"--target-resource-id", cfg.TargetResourceID,
		"--resource-port", cfg.TargetPort,
		"--port", fmt.Sprint(localPort),
	}
	if cfg.SubscriptionID != "" {
		command = append(command, "--subscription", cfg.SubscriptionID)
	}

	conn, err := tunnelcmd.StartAndDial(ctx, command, nil, "127.0.0.1", localPort, 20*time.Second)
	if err != nil {
		return nil, wrapAzureBastionTunnelError(err)
	}
	return conn, nil
}

func wrapAzureBastionTunnelError(err error) error {
	if err == nil {
		return nil
	}

	hint := "hint: Azure Bastion native client/tunnel requires Standard SKU and Native Client Support enabled"
	if strings.Contains(strings.ToLower(err.Error()), "basic") {
		return fmt.Errorf("azure bastion tunnel failed: %w; %s", err, hint)
	}
	return fmt.Errorf("azure bastion tunnel failed: %w; %s", err, hint)
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
