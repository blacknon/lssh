package ssh

import (
	"os"
	"path/filepath"
	"runtime"
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
