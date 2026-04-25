package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blacknon/lssh/providerapi"
)

func TestAzureConnectorRuntimePrefersTargetConfig(t *testing.T) {
	got := azureConnectorRuntime(
		map[string]interface{}{"bastion_runtime": "sdk"},
		map[string]interface{}{"bastion_runtime": "command"},
	)
	if got != "command" {
		t.Fatalf("azureConnectorRuntime() = %q, want command", got)
	}
}

func TestAzureConnectorConfigPrefersTargetConfig(t *testing.T) {
	cfg, err := azureConnectorConfig(
		map[string]interface{}{
			"bastion_name":           "provider-bastion",
			"bastion_resource_group": "provider-rg",
			"subscription_id":        "provider-sub",
		},
		providerapi.ConnectorTarget{
			Config: map[string]interface{}{
				"bastion_name":           "target-bastion",
				"bastion_resource_group": "target-rg",
				"subscription_id":        "target-sub",
			},
			Meta: map[string]string{
				"id": "vm-resource-id",
			},
		},
	)
	if err != nil {
		t.Fatalf("azureConnectorConfig() error = %v", err)
	}
	if cfg.BastionName != "target-bastion" {
		t.Fatalf("BastionName = %q, want target-bastion", cfg.BastionName)
	}
	if cfg.BastionGroup != "target-rg" {
		t.Fatalf("BastionGroup = %q, want target-rg", cfg.BastionGroup)
	}
	if cfg.SubscriptionID != "target-sub" {
		t.Fatalf("SubscriptionID = %q, want target-sub", cfg.SubscriptionID)
	}
}

func TestAzurePowerState(t *testing.T) {
	got := azurePowerState([]azureStatus{
		{Code: "ProvisioningState/succeeded"},
		{Code: "PowerState/running"},
	})
	if got != "running" {
		t.Fatalf("azurePowerState() = %q, want %q", got, "running")
	}
}

func TestAzureResourceGroup(t *testing.T) {
	got := azureResourceGroup("/subscriptions/sub-1/resourceGroups/rg-demo/providers/Microsoft.Compute/virtualMachines/vm1")
	if got != "rg-demo" {
		t.Fatalf("azureResourceGroup() = %q, want %q", got, "rg-demo")
	}
}

func TestRenderAzureTemplate(t *testing.T) {
	vm := azureVM{
		ID:       "/subscriptions/sub-1/resourceGroups/rg-demo/providers/Microsoft.Compute/virtualMachines/vm1",
		Name:     "vm1",
		Location: "japaneast",
		Tags: map[string]string{
			"Role": "web",
		},
	}
	vm.Properties.StorageProfile.OSDisk.OSType = "Linux"

	got := renderAzureTemplate("azure:${name}:${resource_group}:${power_state}:${tags.Role}", vm, "rg-demo", "10.0.0.4", "20.1.2.3", "running")
	want := "azure:vm1:rg-demo:running:web"
	if got != want {
		t.Fatalf("renderAzureTemplate() = %q, want %q", got, want)
	}
}

func TestAzureInventoryFilterDefaultMatchesOnlyRunning(t *testing.T) {
	filter := azureInventoryFilterFromConfig(map[string]interface{}{})
	if !filter.matches("running") {
		t.Fatal("default filter should match running")
	}
	if filter.matches("stopped") {
		t.Fatal("default filter should not match stopped")
	}
}

func TestAzureAddrStrategyDefaultsToPrivateFirst(t *testing.T) {
	if got := azureAddrStrategy(map[string]interface{}{}); got != "private_first" {
		t.Fatalf("azureAddrStrategy() = %q, want private_first", got)
	}
	if got := azureAddrStrategy(map[string]interface{}{"addr_strategy": "unknown"}); got != "private_first" {
		t.Fatalf("azureAddrStrategy(unknown) = %q, want private_first", got)
	}
}

func TestAzureSelectAddress(t *testing.T) {
	tests := []struct {
		name      string
		privateIP string
		publicIP  string
		strategy  string
		want      string
	}{
		{name: "private first", privateIP: "10.0.0.10", publicIP: "20.1.2.3", strategy: "private_first", want: "10.0.0.10"},
		{name: "public first", privateIP: "10.0.0.10", publicIP: "20.1.2.3", strategy: "public_first", want: "20.1.2.3"},
		{name: "private only", privateIP: "10.0.0.10", publicIP: "20.1.2.3", strategy: "private_only", want: "10.0.0.10"},
		{name: "public only", privateIP: "10.0.0.10", publicIP: "20.1.2.3", strategy: "public_only", want: "20.1.2.3"},
		{name: "public first fallback", privateIP: "10.0.0.10", publicIP: "", strategy: "public_first", want: "10.0.0.10"},
		{name: "private first fallback", privateIP: "", publicIP: "20.1.2.3", strategy: "private_first", want: "20.1.2.3"},
		{name: "public only empty", privateIP: "10.0.0.10", publicIP: "", strategy: "public_only", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := azureSelectAddress(tt.privateIP, tt.publicIP, tt.strategy); got != tt.want {
				t.Fatalf("azureSelectAddress() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAzureVMListPathSubscriptionScopeUsesStatusOnly(t *testing.T) {
	got := azureVMListPath("https://management.azure.com", "sub-1", "", true)
	want := "https://management.azure.com/subscriptions/sub-1/providers/Microsoft.Compute/virtualMachines?api-version=" + azureComputeAPIVersion + "&statusOnly=true"
	if got != want {
		t.Fatalf("azureVMListPath() = %q, want %q", got, want)
	}
}

func TestAzureVMListPathResourceGroupScopeIgnoresStatusOnly(t *testing.T) {
	got := azureVMListPath("https://management.azure.com", "sub-1", "rg-demo", true)
	want := "https://management.azure.com/subscriptions/sub-1/resourceGroups/rg-demo/providers/Microsoft.Compute/virtualMachines?api-version=" + azureComputeAPIVersion
	if got != want {
		t.Fatalf("azureVMListPath() = %q, want %q", got, want)
	}
}

func TestAzureSupportsVMStatusList(t *testing.T) {
	if !azureSupportsVMStatusList(map[string]interface{}{}) {
		t.Fatal("azureSupportsVMStatusList() = false, want true without resource_group")
	}
	if azureSupportsVMStatusList(map[string]interface{}{"resource_group": "rg-demo"}) {
		t.Fatal("azureSupportsVMStatusList() = true, want false with resource_group")
	}
}

func TestBuildInventoryEntryFallsBackToInstanceView(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/subscriptions/sub-1/resourceGroups/rg-demo/providers/Microsoft.Compute/virtualMachines/vm1/instanceView" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"statuses":[{"code":"PowerState/running"}]}`))
	}))
	defer server.Close()

	client := &azureClient{
		baseURL:        server.URL,
		httpClient:     server.Client(),
		token:          "test-token",
		subscriptionID: "sub-1",
		nicCache:       map[string]azureNIC{},
		publicIPCache:  map[string]string{},
	}

	vms := []azureVM{
		{ID: "/subscriptions/sub-1/resourceGroups/rg-demo/providers/Microsoft.Compute/virtualMachines/vm1"},
	}
	powerStateByID := map[string]string{}

	serverEntry, _, err := client.buildInventoryEntry(vms[0], powerStateByID, azureInventoryFilter{Statuses: map[string]bool{"running": true}}, "azure:${name}", "", "private_first", nil)
	if err != nil {
		t.Fatalf("buildInventoryEntry() error = %v", err)
	}
	if serverEntry == nil {
		t.Fatal("buildInventoryEntry() = nil, want non-nil")
	}
	if got := serverEntry.Meta["power_state"]; got != "running" {
		t.Fatalf("serverEntry.Meta[power_state] = %q, want running", got)
	}
}

func TestBuildInventoryEntryNetworkStateRunsStatusAndNetworkInParallel(t *testing.T) {
	var current int32
	var maxConcurrent int32

	recordStart := func() {
		active := atomic.AddInt32(&current, 1)
		for {
			seen := atomic.LoadInt32(&maxConcurrent)
			if active <= seen || atomic.CompareAndSwapInt32(&maxConcurrent, seen, active) {
				return
			}
		}
	}
	recordDone := func() {
		atomic.AddInt32(&current, -1)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recordStart()
		defer recordDone()
		time.Sleep(50 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/subscriptions/sub-1/resourceGroups/rg-demo/providers/Microsoft.Compute/virtualMachines/vm1/instanceView":
			_, _ = w.Write([]byte(`{"statuses":[{"code":"PowerState/running"}]}`))
		case "/subscriptions/sub-1/resourceGroups/rg-demo/providers/Microsoft.Network/networkInterfaces/nic-1":
			_, _ = w.Write([]byte(`{"properties":{"ipConfigurations":[{"properties":{"privateIPAddress":"10.0.0.4","publicIPAddress":{"id":"/subscriptions/sub-1/resourceGroups/rg-demo/providers/Microsoft.Network/publicIPAddresses/pip-1"}}}]}}`))
		case "/subscriptions/sub-1/resourceGroups/rg-demo/providers/Microsoft.Network/publicIPAddresses/pip-1":
			_, _ = w.Write([]byte(`{"properties":{"ipAddress":"20.1.2.3"}}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := &azureClient{
		baseURL:        server.URL,
		httpClient:     server.Client(),
		token:          "test-token",
		subscriptionID: "sub-1",
		nicCache:       map[string]azureNIC{},
		publicIPCache:  map[string]string{},
	}
	vm := azureVM{ID: "/subscriptions/sub-1/resourceGroups/rg-demo/providers/Microsoft.Compute/virtualMachines/vm1"}
	vm.Properties.NetworkProfile.NetworkInterfaces = []struct {
		ID string `json:"id"`
	}{
		{ID: "/subscriptions/sub-1/resourceGroups/rg-demo/providers/Microsoft.Network/networkInterfaces/nic-1"},
	}

	start := time.Now()
	powerState, privateIP, publicIP, ipErr, err := client.buildInventoryEntryNetworkState(vm, "")
	if err != nil {
		t.Fatalf("buildInventoryEntryNetworkState() error = %v", err)
	}
	if ipErr != nil {
		t.Fatalf("buildInventoryEntryNetworkState() ipErr = %v", ipErr)
	}
	if powerState != "running" || privateIP != "10.0.0.4" || publicIP != "20.1.2.3" {
		t.Fatalf("unexpected network state: power=%q private=%q public=%q", powerState, privateIP, publicIP)
	}
	if atomic.LoadInt32(&maxConcurrent) < 2 {
		t.Fatalf("max concurrent azure requests = %d, want at least 2", maxConcurrent)
	}
	if elapsed := time.Since(start); elapsed >= 140*time.Millisecond {
		t.Fatalf("buildInventoryEntryNetworkState() took %v, want parallel request overlap", elapsed)
	}
}

func TestAzureInventoryBuildParallelismConstant(t *testing.T) {
	if azureInventoryBuildParallelism < 4 {
		t.Fatalf("azureInventoryBuildParallelism = %d, want at least 4", azureInventoryBuildParallelism)
	}
}

func TestAzureTokenScopeDefault(t *testing.T) {
	if got := azureTokenScope(map[string]interface{}{}); got != "https://management.azure.com/.default" {
		t.Fatalf("azureTokenScope() = %q, want default ARM scope", got)
	}
}

func TestAzureTokenScopeCustomEndpoint(t *testing.T) {
	if got := azureTokenScope(map[string]interface{}{"endpoint": "https://management.usgovcloudapi.net/"}); got != "https://management.usgovcloudapi.net/.default" {
		t.Fatalf("azureTokenScope(custom) = %q, want custom ARM scope", got)
	}
}

func TestAzureSubscriptionIDFromConfig(t *testing.T) {
	got, err := azureSubscriptionID(t.Context(), map[string]interface{}{"subscription_id": "sub-1"}, "https://management.azure.com", "token")
	if err != nil {
		t.Fatalf("azureSubscriptionID() error = %v", err)
	}
	if got != "sub-1" {
		t.Fatalf("azureSubscriptionID() = %q, want %q", got, "sub-1")
	}
}

func TestAzureSubscriptionIDFromEnv(t *testing.T) {
	t.Setenv("AZURE_SUBSCRIPTION_ID", "sub-env")
	got, err := azureSubscriptionID(t.Context(), map[string]interface{}{}, "https://management.azure.com", "token")
	if err != nil {
		t.Fatalf("azureSubscriptionID() error = %v", err)
	}
	if got != "sub-env" {
		t.Fatalf("azureSubscriptionID() = %q, want %q", got, "sub-env")
	}
}

func TestAzureSubscriptionIDRequiresTokenOrSetting(t *testing.T) {
	original := os.Getenv("AZURE_SUBSCRIPTION_ID")
	t.Setenv("AZURE_SUBSCRIPTION_ID", "")
	defer os.Setenv("AZURE_SUBSCRIPTION_ID", original)

	if _, err := azureSubscriptionID(t.Context(), map[string]interface{}{}, "https://management.azure.com", ""); err == nil {
		t.Fatal("azureSubscriptionID() error = nil, want non-nil")
	}
}

func TestAzureSelectSubscriptionID(t *testing.T) {
	tests := []struct {
		name          string
		subscriptions []struct {
			SubscriptionID string `json:"subscriptionId"`
			State          string `json:"state"`
		}
		want    string
		wantErr bool
	}{
		{
			name: "single enabled",
			subscriptions: []struct {
				SubscriptionID string `json:"subscriptionId"`
				State          string `json:"state"`
			}{
				{SubscriptionID: "sub-1", State: "Enabled"},
			},
			want: "sub-1",
		},
		{
			name: "single empty state",
			subscriptions: []struct {
				SubscriptionID string `json:"subscriptionId"`
				State          string `json:"state"`
			}{
				{SubscriptionID: "sub-1", State: ""},
			},
			want: "sub-1",
		},
		{
			name: "multiple enabled",
			subscriptions: []struct {
				SubscriptionID string `json:"subscriptionId"`
				State          string `json:"state"`
			}{
				{SubscriptionID: "sub-1", State: "Enabled"},
				{SubscriptionID: "sub-2", State: "Enabled"},
			},
			wantErr: true,
		},
		{
			name: "none enabled",
			subscriptions: []struct {
				SubscriptionID string `json:"subscriptionId"`
				State          string `json:"state"`
			}{
				{SubscriptionID: "sub-1", State: "Disabled"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := azureSelectSubscriptionID(tt.subscriptions)
			if tt.wantErr {
				if err == nil {
					t.Fatal("azureSelectSubscriptionID() error = nil, want non-nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("azureSelectSubscriptionID() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("azureSelectSubscriptionID() = %q, want %q", got, tt.want)
			}
		})
	}
}
