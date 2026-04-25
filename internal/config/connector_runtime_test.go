package conf

import (
	"testing"

	"github.com/blacknon/lssh/providerapi"
)

func TestManagedSSHPlanFromProviderSupportsSDKSSHTransport(t *testing.T) {
	t.Parallel()

	managed := managedSSHPlanFromProvider(providerapi.ConnectorPlan{
		Kind: "provider-managed",
		Details: map[string]interface{}{
			"ssh_runtime": "sdk",
			"transport":   "ssh_transport",
		},
	}, ConnectorOperation{Name: "shell"})
	if managed == nil {
		t.Fatal("managedSSHPlanFromProvider() = nil, want sdk ssh transport")
	}
}

func TestManagedSSHPlanFromProviderSupportsNativeShellRuntimeForLocalRC(t *testing.T) {
	t.Parallel()

	managed := managedSSHPlanFromProvider(providerapi.ConnectorPlan{
		Kind: "provider-managed",
		Details: map[string]interface{}{
			"shell_runtime":  "native",
			"session_action": "start",
		},
	}, ConnectorOperation{Name: "shell"})
	if managed != nil {
		t.Fatal("managedSSHPlanFromProvider() != nil, want nil for native shell runtime")
	}
}

func TestManagedSSHPlanFromProviderRejectsNativeAttachShellForLocalRC(t *testing.T) {
	t.Parallel()

	managed := managedSSHPlanFromProvider(providerapi.ConnectorPlan{
		Kind: "provider-managed",
		Details: map[string]interface{}{
			"shell_runtime":  "native",
			"session_action": "attach",
		},
	}, ConnectorOperation{Name: "shell"})
	if managed != nil {
		t.Fatal("managedSSHPlanFromProvider() != nil, want nil for attach shell")
	}
}

func TestProviderManagedSupportsLocalRCForNativeStartShell(t *testing.T) {
	t.Parallel()

	if !providerManagedSupportsLocalRC(providerapi.ConnectorPlan{
		Kind: "provider-managed",
		Details: map[string]interface{}{
			"shell_runtime":  "native",
			"session_action": "start",
		},
	}, ConnectorOperation{Name: "shell"}) {
		t.Fatal("providerManagedSupportsLocalRC() = false, want true")
	}
}

func TestProviderManagedSharesShellForwardForNativeStartShell(t *testing.T) {
	t.Parallel()

	if !providerManagedSharesShellForward(providerapi.ConnectorPlan{
		Kind: "provider-managed",
		Details: map[string]interface{}{
			"shell_runtime":  "native",
			"session_action": "start",
		},
	}, ConnectorOperation{Name: "shell"}) {
		t.Fatal("providerManagedSharesShellForward() = false, want true")
	}
}
