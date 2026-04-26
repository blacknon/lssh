package eiceconnector

import (
	"errors"
	"strings"
	"testing"
)

func TestWrapAWSEICETunnelErrorAddsHint(t *testing.T) {
	err := wrapAWSEICETunnelError(errors.New("tunnel command exited before ready"))
	if err == nil {
		t.Fatal("wrapAWSEICETunnelError() = nil, want non-nil")
	}
	message := err.Error()
	if !strings.Contains(message, "verify the EC2 Instance Connect Endpoint ID/DNS") {
		t.Fatalf("error = %q, want EICE hint", message)
	}
}
