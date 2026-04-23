package ssh

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestConnectorLocalForwardSpecLocalhostTCP(t *testing.T) {
	spec, err := connectorLocalForwardSpec(conf.ServerConfig{
		Forwards: []*conf.PortForward{
			{
				Mode:          "L",
				LocalNetwork:  "tcp",
				Local:         "localhost:15432",
				RemoteNetwork: "tcp",
				Remote:        "db.internal:5432",
			},
		},
	})
	if err != nil {
		t.Fatalf("connectorLocalForwardSpec() error = %v", err)
	}
	if spec.ListenPort != "15432" {
		t.Fatalf("ListenPort = %q, want 15432", spec.ListenPort)
	}
	if spec.TargetHost != "db.internal" {
		t.Fatalf("TargetHost = %q, want db.internal", spec.TargetHost)
	}
}

func TestConnectorLocalForwardSpecRejectsMultipleForwards(t *testing.T) {
	_, err := connectorLocalForwardSpec(conf.ServerConfig{
		Forwards: []*conf.PortForward{
			{Mode: "L", Local: "localhost:8080", Remote: "localhost:80"},
			{Mode: "L", Local: "localhost:8081", Remote: "localhost:81"},
		},
	})
	if err == nil {
		t.Fatal("connectorLocalForwardSpec() error = nil, want non-nil")
	}
}

func TestConnectorForwardModeDynamic(t *testing.T) {
	mode, err := connectorForwardMode(conf.ServerConfig{
		DynamicPortForward: "1080",
	})
	if err != nil {
		t.Fatalf("connectorForwardMode() error = %v", err)
	}
	if mode != "dynamic" {
		t.Fatalf("connectorForwardMode() = %q, want dynamic", mode)
	}
}

func TestConnectorForwardModeRejectsUnsupportedReverseDynamic(t *testing.T) {
	_, err := connectorForwardMode(conf.ServerConfig{
		ReverseDynamicPortForward: "2080",
	})
	if err == nil {
		t.Fatal("connectorForwardMode() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "-R port") {
		t.Fatalf("connectorForwardMode() error = %q, want reverse dynamic hint", err)
	}
}

func TestConnectorDynamicForwardSpecRejectsInvalidPort(t *testing.T) {
	_, err := connectorDynamicForwardSpec(conf.ServerConfig{
		DynamicPortForward: "localhost",
	})
	if err == nil {
		t.Fatal("connectorDynamicForwardSpec() error = nil, want non-nil")
	}
}

func TestAWSSSMDynamicDialNeedsPluginFallback(t *testing.T) {
	if !awsSSMDynamicDialNeedsPluginFallback(providerapi.ConnectorPlan{
		Details: map[string]interface{}{
			"connector":    "aws-ssm",
			"session_mode": "tcp-dial-transport",
		},
	}) {
		t.Fatal("awsSSMDynamicDialNeedsPluginFallback() = false, want true for aws-ssm tcp-dial-transport")
	}

	if awsSSMDynamicDialNeedsPluginFallback(providerapi.ConnectorPlan{
		Details: map[string]interface{}{
			"connector":    "aws-ssm",
			"session_mode": "port-forward-local",
		},
	}) {
		t.Fatal("awsSSMDynamicDialNeedsPluginFallback() = true, want false for local port forward")
	}
}

func TestConnectorForwardingSharesShellForAWSSSMDynamicOnly(t *testing.T) {
	r := &Run{}
	if !r.connectorForwardingSharesShell(conf.ServerConfig{DynamicPortForward: "11080"}, "aws-ssm") {
		t.Fatal("connectorForwardingSharesShell() = false, want true for aws-ssm dynamic forward")
	}
	if r.connectorForwardingSharesShell(conf.ServerConfig{DynamicPortForward: "11080"}, "winrm") {
		t.Fatal("connectorForwardingSharesShell() = true, want false for non aws-ssm connector")
	}
	if r.connectorForwardingSharesShell(conf.ServerConfig{
		DynamicPortForward: "11080",
		Forwards:           []*conf.PortForward{{Mode: "L", Local: "localhost:15432", Remote: "db.internal:5432"}},
	}, "aws-ssm") {
		t.Fatal("connectorForwardingSharesShell() = true, want false when local forward is also configured")
	}
	if (&Run{IsNone: true}).connectorForwardingSharesShell(conf.ServerConfig{DynamicPortForward: "11080"}, "aws-ssm") {
		t.Fatal("connectorForwardingSharesShell() = true, want false with --not-execute")
	}
}

func TestRunConnectorWithPrePost(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pre_cmd/post_cmd helper test uses sh-compatible commands")
	}

	tempDir := t.TempDir()
	prePath := filepath.Join(tempDir, "pre.txt")
	postPath := filepath.Join(tempDir, "post.txt")

	err := runConnectorWithPrePost(conf.ServerConfig{
		PreCmd:  "printf pre > " + prePath,
		PostCmd: "printf post > " + postPath,
	}, func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("runConnectorWithPrePost() error = %v", err)
	}

	preData, err := os.ReadFile(prePath)
	if err != nil {
		t.Fatalf("ReadFile(pre) error = %v", err)
	}
	if string(preData) != "pre" {
		t.Fatalf("pre data = %q, want %q", string(preData), "pre")
	}

	postData, err := os.ReadFile(postPath)
	if err != nil {
		t.Fatalf("ReadFile(post) error = %v", err)
	}
	if string(postData) != "post" {
		t.Fatalf("post data = %q, want %q", string(postData), "post")
	}
}
