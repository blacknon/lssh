package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blacknon/lssh/internal/providerapi"
	"github.com/blacknon/lssh/internal/providerbuiltin"
	"github.com/blacknon/lssh/provider/connector/provider-connector-telnet/telnetlib"
)

func main() {
	req, err := providerbuiltin.ReadRequest()
	if err != nil {
		_ = providerbuiltin.WriteError(err.Error())
		os.Exit(1)
	}

	switch req.Method {
	case providerapi.MethodPluginDescribe:
		_ = providerbuiltin.WriteResponse(req, providerapi.PluginDescribeResult{
			Name:            "provider-connector-telnet",
			Capabilities:    []string{"connector"},
			ConnectorNames:  []string{"telnet"},
			Methods:         []string{providerapi.MethodPluginDescribe, providerapi.MethodHealthCheck, providerapi.MethodConnectorDescribe, providerapi.MethodConnectorPrepare},
			ProtocolVersion: providerapi.Version,
		}, nil)
	case providerapi.MethodHealthCheck:
		var params providerapi.HealthCheckParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := telnetHealthCheck(params.Config)
		if err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "health_check_failed", err.Error())
			os.Exit(1)
		}
		_ = providerbuiltin.WriteResponse(req, result, nil)
	case providerapi.MethodConnectorDescribe:
		var params providerapi.ConnectorDescribeParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := telnetDescribe(params)
		if err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "connector_describe_failed", err.Error())
			os.Exit(1)
		}
		_ = providerbuiltin.WriteResponse(req, result, nil)
	case providerapi.MethodConnectorPrepare, providerapi.MethodTransportPrep:
		var params providerapi.ConnectorPrepareParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := telnetPrepare(params)
		if err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "connector_prepare_failed", err.Error())
			os.Exit(1)
		}
		_ = providerbuiltin.WriteResponse(req, result, nil)
	default:
		_ = providerbuiltin.WriteErrorResponse(req, "unsupported_method", fmt.Sprintf("unsupported method %q", req.Method))
		os.Exit(1)
	}
}

func decodeParams(raw interface{}, out interface{}) error {
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func telnetHealthCheck(config map[string]interface{}) (providerapi.HealthCheckResult, error) {
	if providerbuiltin.String(config, "addr") != "" {
		if _, err := telnetlib.ConfigFromMaps(config, nil); err != nil {
			return providerapi.HealthCheckResult{}, err
		}
	}
	return providerapi.HealthCheckResult{
		OK:      true,
		Message: "telnet connector configuration looks valid",
	}, nil
}

func telnetDescribe(params providerapi.ConnectorDescribeParams) (providerapi.ConnectorDescribeResult, error) {
	cfg, _ := telnetlib.ConfigFromMaps(params.Config, params.Target.Config)
	addr := cfg.Host
	capabilities := map[string]providerapi.ConnectorCapability{
		"shell": {
			Supported: addr != "",
			Reason:    unsupportedReason(addr != "", "target addr is required for telnet shell access"),
		},
		"exec": {
			Supported: false,
			Reason:    "telnet does not expose a distinct non-interactive exec primitive",
		},
		"exec_pty": {
			Supported: false,
			Reason:    "telnet shell access is already line-oriented and should not be treated as a PTY exec capability",
		},
		"upload": {
			Supported: false,
			Reason:    "telnet does not provide native file upload support",
		},
		"download": {
			Supported: false,
			Reason:    "telnet does not provide native file download support",
		},
		"port_forward_local": {
			Supported: false,
			Reason:    "telnet does not provide native local port forwarding",
		},
		"port_forward_remote": {
			Supported: false,
			Reason:    "telnet does not provide native remote port forwarding",
		},
		"mount": {
			Supported: false,
			Reason:    "telnet does not provide mount semantics",
		},
		"agent_forward": {
			Supported: false,
			Reason:    "telnet does not provide agent forwarding",
		},
	}
	return providerapi.ConnectorDescribeResult{Capabilities: capabilities}, nil
}

func telnetPrepare(params providerapi.ConnectorPrepareParams) (providerapi.ConnectorPrepareResult, error) {
	cfg, err := telnetlib.ConfigFromMaps(params.Config, params.Target.Config)
	if err != nil {
		return providerapi.ConnectorPrepareResult{}, err
	}

	switch params.Operation.Name {
	case "shell":
		return providerapi.ConnectorPrepareResult{
			Supported: true,
			Plan: providerapi.ConnectorPlan{
				Kind: "provider-managed",
				Details: map[string]interface{}{
					"connector":           "telnet",
					"addr":                cfg.Host,
					"port":                cfg.Port,
					"user":                cfg.Username,
					"pass":                cfg.Password,
					"login_prompt":        cfg.LoginPrompt,
					"password_prompt":     cfg.PasswordPrompt,
					"initial_newline":     cfg.InitialNewline,
					"dial_timeout_sec":    cfg.DialTimeoutSec,
					"post_login_delay_ms": cfg.PostLoginDelayMs,
				},
			},
		}, nil
	default:
		return providerapi.ConnectorPrepareResult{
			Supported: false,
			Plan: providerapi.ConnectorPlan{
				Kind: "provider-managed",
				Details: map[string]interface{}{
					"connector": "telnet",
					"reason":    fmt.Sprintf("operation %q is not supported by the telnet connector", params.Operation.Name),
				},
			},
		}, nil
	}
}

func unsupportedReason(supported bool, reason string) string {
	if supported {
		return ""
	}
	return reason
}
