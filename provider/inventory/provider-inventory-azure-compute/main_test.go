package main

import "testing"

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
