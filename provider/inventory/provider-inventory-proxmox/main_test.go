package main

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

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

func TestProxmoxResourceOSTypesWithLookupRunsInParallel(t *testing.T) {
	resources := []proxmoxClusterResource{
		{ID: "qemu/101", Node: "node-a", Type: "qemu", VMID: 101},
		{ID: "qemu/102", Node: "node-a", Type: "qemu", VMID: 102},
		{ID: "qemu/103", Node: "node-a", Type: "qemu", VMID: 103},
		{ID: "qemu/104", Node: "node-a", Type: "qemu", VMID: 104},
	}

	var current int32
	var maxConcurrent int32
	lookup := func(resource proxmoxClusterResource) (string, error) {
		active := atomic.AddInt32(&current, 1)
		for {
			seen := atomic.LoadInt32(&maxConcurrent)
			if active <= seen || atomic.CompareAndSwapInt32(&maxConcurrent, seen, active) {
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&current, -1)
		return fmt.Sprintf("linux-%d", resource.VMID), nil
	}

	start := time.Now()
	got, err := proxmoxResourceOSTypesWithLookup(resources, map[string]string{"node-a": "online"}, lookup)
	if err != nil {
		t.Fatalf("proxmoxResourceOSTypesWithLookup() error = %v", err)
	}
	if len(got) != len(resources) {
		t.Fatalf("got %d ostypes, want %d", len(got), len(resources))
	}
	if atomic.LoadInt32(&maxConcurrent) < 2 {
		t.Fatalf("max concurrent lookups = %d, want at least 2", maxConcurrent)
	}
	if elapsed := time.Since(start); elapsed >= 180*time.Millisecond {
		t.Fatalf("lookup took %v, want parallel execution", elapsed)
	}
}

func TestProxmoxResourceOSTypesWithLookupSkipsOfflineNodes(t *testing.T) {
	resources := []proxmoxClusterResource{
		{ID: "qemu/101", Node: "node-a", Type: "qemu", VMID: 101},
		{ID: "qemu/102", Node: "node-b", Type: "qemu", VMID: 102},
	}

	var calls int32
	got, err := proxmoxResourceOSTypesWithLookup(resources, map[string]string{
		"node-a": "online",
		"node-b": "offline",
	}, func(resource proxmoxClusterResource) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "linux", nil
	})
	if err != nil {
		t.Fatalf("proxmoxResourceOSTypesWithLookup() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("lookup calls = %d, want 1", calls)
	}
	if _, ok := got["qemu/102"]; ok {
		t.Fatalf("offline node result should not be included: %#v", got)
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
