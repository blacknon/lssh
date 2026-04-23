package ssh

import (
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/providerapi"
)

func TestConnectorLocalRCEnabledDefaultConfig(t *testing.T) {
	if !connectorLocalRCEnabled(&Run{}, conf.ServerConfig{LocalRcUse: "yes"}) {
		t.Fatal("connectorLocalRCEnabled() = false, want true when config enables localrc")
	}
}

func TestConnectorLocalRCEnabledRespectsNotLocalRC(t *testing.T) {
	if connectorLocalRCEnabled(&Run{IsNotBashrc: true}, conf.ServerConfig{LocalRcUse: "yes"}) {
		t.Fatal("connectorLocalRCEnabled() = true, want false with --not-localrc")
	}
}

func TestConnectorLocalRCEnabledRespectsLocalRCFlag(t *testing.T) {
	if !connectorLocalRCEnabled(&Run{IsBashrc: true}, conf.ServerConfig{LocalRcUse: "no"}) {
		t.Fatal("connectorLocalRCEnabled() = false, want true with --localrc")
	}
}

func TestConnectorPlanSupportsLocalRCForAWSNativeOnly(t *testing.T) {
	nativePlan := providerapi.ConnectorPlan{
		Details: map[string]interface{}{
			"connector":      "aws-ssm",
			"shell_runtime":  "native",
			"session_action": "start",
		},
	}
	if !connectorPlanSupportsLocalRC(nativePlan, "aws-ssm") {
		t.Fatal("connectorPlanSupportsLocalRC(native aws-ssm) = false, want true")
	}

	pluginPlan := providerapi.ConnectorPlan{
		Details: map[string]interface{}{
			"connector":      "aws-ssm",
			"shell_runtime":  "plugin",
			"session_action": "start",
		},
	}
	if connectorPlanSupportsLocalRC(pluginPlan, "aws-ssm") {
		t.Fatal("connectorPlanSupportsLocalRC(plugin aws-ssm) = true, want false")
	}
}
