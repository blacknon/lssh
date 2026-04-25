package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGCPConnectorRuntimePrefersTargetConfig(t *testing.T) {
	got := gcpConnectorRuntime(
		map[string]interface{}{"iap_runtime": "sdk"},
		map[string]interface{}{"iap_runtime": "command"},
	)
	if got != "command" {
		t.Fatalf("gcpConnectorRuntime() = %q, want command", got)
	}
}

func TestGCPCredentialsFileExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}

	got := gcpCredentialsFile(map[string]interface{}{
		"credentials_file": "~/.config/gcloud/application_default_credentials.json",
	})
	want := filepath.Join(home, ".config/gcloud/application_default_credentials.json")
	if got != want {
		t.Fatalf("gcpCredentialsFile() = %q, want %q", got, want)
	}
}

func TestGCPAddrStrategyDefaultsToPrivateFirst(t *testing.T) {
	if got := gcpAddrStrategy(map[string]interface{}{}); got != "private_first" {
		t.Fatalf("gcpAddrStrategy() = %q, want private_first", got)
	}
	if got := gcpAddrStrategy(map[string]interface{}{"addr_strategy": "unknown"}); got != "private_first" {
		t.Fatalf("gcpAddrStrategy(unknown) = %q, want private_first", got)
	}
}

func TestGCPSelectAddress(t *testing.T) {
	tests := []struct {
		name      string
		privateIP string
		publicIP  string
		strategy  string
		want      string
	}{
		{name: "private first", privateIP: "10.0.0.10", publicIP: "34.0.0.10", strategy: "private_first", want: "10.0.0.10"},
		{name: "public first", privateIP: "10.0.0.10", publicIP: "34.0.0.10", strategy: "public_first", want: "34.0.0.10"},
		{name: "private only", privateIP: "10.0.0.10", publicIP: "34.0.0.10", strategy: "private_only", want: "10.0.0.10"},
		{name: "public only", privateIP: "10.0.0.10", publicIP: "34.0.0.10", strategy: "public_only", want: "34.0.0.10"},
		{name: "public first fallback", privateIP: "10.0.0.10", publicIP: "", strategy: "public_first", want: "10.0.0.10"},
		{name: "private first fallback", privateIP: "", publicIP: "34.0.0.10", strategy: "private_first", want: "34.0.0.10"},
		{name: "public only empty", privateIP: "10.0.0.10", publicIP: "", strategy: "public_only", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gcpSelectAddress(tt.privateIP, tt.publicIP, tt.strategy); got != tt.want {
				t.Fatalf("gcpSelectAddress() = %q, want %q", got, tt.want)
			}
		})
	}
}
