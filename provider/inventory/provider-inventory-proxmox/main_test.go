package main

import "testing"

func TestProxmoxInventoryFilterFromConfigDefaults(t *testing.T) {
	filter := proxmoxInventoryFilterFromConfig(map[string]interface{}{})
	if !filter.Statuses["running"] || len(filter.Statuses) != 1 {
		t.Fatalf("statuses = %#v", filter.Statuses)
	}
	if filter.IncludeTemplates {
		t.Fatal("include templates should default to false")
	}
}

func TestProxmoxInventoryFilterFromConfigCustomFilters(t *testing.T) {
	filter := proxmoxInventoryFilterFromConfig(map[string]interface{}{
		"statuses":    []interface{}{"running", "stopped"},
		"vm_types":    []interface{}{"qemu"},
		"os_families": []interface{}{"windows"},
	})
	if !filter.Statuses["running"] || !filter.Statuses["stopped"] {
		t.Fatalf("statuses = %#v", filter.Statuses)
	}
	if !filter.VMTypes["qemu"] || len(filter.VMTypes) != 1 {
		t.Fatalf("vm types = %#v", filter.VMTypes)
	}
	if !filter.OSFamilies["windows"] || len(filter.OSFamilies) != 1 {
		t.Fatalf("os families = %#v", filter.OSFamilies)
	}
}

func TestProxmoxOSFamily(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		ostype       string
		want         string
	}{
		{name: "qemu windows", resourceType: "qemu", ostype: "win10", want: "windows"},
		{name: "qemu linux", resourceType: "qemu", ostype: "l26", want: "non-windows"},
		{name: "lxc", resourceType: "lxc", ostype: "", want: "non-windows"},
	}

	for _, tt := range tests {
		if got := proxmoxOSFamily(tt.resourceType, tt.ostype); got != tt.want {
			t.Fatalf("%s: os family = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestProxmoxOSFamilyUnknownQEMUIsNotForcedToNonWindows(t *testing.T) {
	if got := proxmoxOSFamily("qemu", ""); got != "unknown" {
		t.Fatalf("os family = %q, want unknown", got)
	}
}

func TestProxmoxInventoryFilterMatches(t *testing.T) {
	filter := proxmoxInventoryFilter{
		Statuses:   map[string]bool{"running": true},
		VMTypes:    map[string]bool{"qemu": true},
		OSFamilies: map[string]bool{"windows": true},
	}
	resource := proxmoxClusterResource{
		Status: "running",
		Type:   "qemu",
	}
	if !filter.matches(resource, "windows") {
		t.Fatal("expected resource to match")
	}
	if filter.matches(resource, "non-windows") {
		t.Fatal("did not expect non-windows resource to match")
	}
}

func TestProxmoxNodeOfflineSkipsOSTypeLookup(t *testing.T) {
	resource := proxmoxClusterResource{
		ID:   "qemu/10082",
		Node: "sv-pve00",
		Type: "qemu",
	}
	nodeStatuses := map[string]string{"sv-pve00": "offline"}
	if nodeStatuses[resource.Node] != "offline" {
		t.Fatalf("node status = %q", nodeStatuses[resource.Node])
	}
}

func TestProxmoxRequestPath(t *testing.T) {
	got := proxmoxRequestPath("/api2/json/cluster/resources", "type=vm")
	want := "/api2/json/cluster/resources?type=vm"
	if got != want {
		t.Fatalf("request path = %q, want %q", got, want)
	}
}

func TestProxmoxHTTPErrorIncludesStatusPathAndBody(t *testing.T) {
	err := proxmoxHTTPError("proxmox api request failed", "403 Forbidden", "/api2/json/cluster/resources", []byte(`{"data":null}`))
	want := `proxmox api request failed: status=403 Forbidden path=/api2/json/cluster/resources body={"data":null}`
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

func TestProxmoxHTTPErrorWithoutBody(t *testing.T) {
	err := proxmoxHTTPError("proxmox auth failed", "401 Unauthorized", "/api2/json/access/ticket", nil)
	want := `proxmox auth failed: status=401 Unauthorized path=/api2/json/access/ticket`
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}
