package providerapi

import (
	"encoding/json"
	"testing"
)

func TestConnectorDescribeResultJSON(t *testing.T) {
	result := ConnectorDescribeResult{
		Capabilities: map[string]ConnectorCapability{
			"shell": {
				Supported: true,
				Requires:  []string{"aws:ssm_agent"},
				Preferred: true,
			},
			"upload": {
				Supported: false,
				Reason:    "not supported in the first design pass",
			},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	capabilities, ok := decoded["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("capabilities = %#v", decoded["capabilities"])
	}

	shell, ok := capabilities["shell"].(map[string]interface{})
	if !ok {
		t.Fatalf("shell = %#v", capabilities["shell"])
	}
	if supported, _ := shell["supported"].(bool); !supported {
		t.Fatalf("shell.supported = %#v, want true", shell["supported"])
	}
}

func TestConnectorPrepareResultJSON(t *testing.T) {
	result := ConnectorPrepareResult{
		Supported: true,
		Plan: ConnectorPlan{
			Kind:    "command",
			Program: "aws",
			Args:    []string{"ssm", "start-session"},
			Env: map[string]string{
				"AWS_PROFILE": "default",
			},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded ConnectorPrepareResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if !decoded.Supported {
		t.Fatal("decoded.Supported = false, want true")
	}
	if decoded.Plan.Kind != "command" {
		t.Fatalf("decoded.Plan.Kind = %q, want %q", decoded.Plan.Kind, "command")
	}
	if decoded.Plan.Program != "aws" {
		t.Fatalf("decoded.Plan.Program = %q, want %q", decoded.Plan.Program, "aws")
	}
}
