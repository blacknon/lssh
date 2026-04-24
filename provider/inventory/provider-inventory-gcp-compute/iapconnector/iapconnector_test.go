package iapconnector

import (
	"context"
	"strings"
	"testing"

	"github.com/blacknon/lssh/internal/providerapi"
)

func TestConfigFromPlanIncludesPureSDKFields(t *testing.T) {
	plan := providerapi.ConnectorPlan{
		Details: map[string]interface{}{
			"project":          "example-project",
			"zone":             "asia-northeast1-b",
			"instance_name":    "test-vm",
			"interface":        "nic0",
			"target_port":      "22",
			"credentials_file": "/tmp/adc.json",
			"scopes":           []interface{}{"scope-a", "scope-b"},
		},
	}

	cfg, err := ConfigFromPlan(plan)
	if err != nil {
		t.Fatalf("ConfigFromPlan() error = %v", err)
	}
	if cfg.CredentialsFile != "/tmp/adc.json" {
		t.Fatalf("CredentialsFile = %q, want /tmp/adc.json", cfg.CredentialsFile)
	}
	if len(cfg.Scopes) != 2 || cfg.Scopes[0] != "scope-a" || cfg.Scopes[1] != "scope-b" {
		t.Fatalf("Scopes = %#v, want [scope-a scope-b]", cfg.Scopes)
	}
}

func TestIAPConnectURLUsesInstanceTarget(t *testing.T) {
	rawURL, err := iapConnectURL(Config{
		Project:      "example-project",
		Zone:         "asia-northeast1-b",
		InstanceName: "test-vm",
		Interface:    "nic0",
		TargetPort:   "22",
	})
	if err != nil {
		t.Fatalf("iapConnectURL() error = %v", err)
	}

	for _, want := range []string{
		"project=example-project",
		"zone=asia-northeast1-b",
		"instance=test-vm",
		"interface=nic0",
		"port=22",
		"newWebsocket=true",
	} {
		if !strings.Contains(rawURL, want) {
			t.Fatalf("iapConnectURL() = %q, want substring %q", rawURL, want)
		}
	}
}

func TestWaitForConnectHonorsContext(t *testing.T) {
	conn := &iapConn{
		connectCh: make(chan error),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := conn.waitForConnect(ctx); err == nil {
		t.Fatal("waitForConnect() error = nil, want context error")
	}
}
