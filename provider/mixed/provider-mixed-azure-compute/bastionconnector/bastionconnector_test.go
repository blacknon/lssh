package bastionconnector

import (
	"errors"
	"strings"
	"testing"
)

func TestWrapAzureBastionTunnelErrorAddsHint(t *testing.T) {
	err := wrapAzureBastionTunnelError(errors.New("tunnel command exited before ready"))
	if err == nil {
		t.Fatal("wrapAzureBastionTunnelError() = nil, want non-nil")
	}
	message := err.Error()
	if !strings.Contains(message, "Azure Bastion native client/tunnel requires Standard SKU and Native Client Support enabled") {
		t.Fatalf("error = %q, want native client hint", message)
	}
}
