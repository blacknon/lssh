package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/blacknon/lssh/provider/connector/provider-connector-winrm/winrmlib"
	"github.com/blacknon/lssh/providerapi"
)

func main() {
	req, err := providerapi.ReadRequest()
	if err != nil {
		_ = providerapi.WriteError(err.Error())
		os.Exit(1)
	}

	switch req.Method {
	case providerapi.MethodPluginDescribe:
		_ = providerapi.WriteResponse(req, providerapi.PluginDescribeResult{
			Name:            "provider-connector-winrm",
			Capabilities:    []string{"connector"},
			ConnectorNames:  []string{"winrm"},
			Methods:         []string{providerapi.MethodPluginDescribe, providerapi.MethodHealthCheck, providerapi.MethodConnectorDescribe, providerapi.MethodConnectorPrepare, providerapi.MethodConnectorExec},
			ProtocolVersion: providerapi.Version,
		}, nil)
	case providerapi.MethodHealthCheck:
		var params providerapi.HealthCheckParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := winrmHealthCheck(params.Config)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "health_check_failed", err.Error())
			os.Exit(1)
		}
		_ = providerapi.WriteResponse(req, result, nil)
	case providerapi.MethodConnectorDescribe:
		var params providerapi.ConnectorDescribeParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := winrmDescribe(params)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "connector_describe_failed", err.Error())
			os.Exit(1)
		}
		_ = providerapi.WriteResponse(req, result, nil)
	case providerapi.MethodConnectorPrepare, providerapi.MethodTransportPrep:
		var params providerapi.ConnectorPrepareParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := winrmPrepare(params)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "connector_prepare_failed", err.Error())
			os.Exit(1)
		}
		_ = providerapi.WriteResponse(req, result, nil)
	case providerapi.MethodConnectorExec:
		var params providerapi.ConnectorRuntimeParams
		if err := decodeParams(req.Params, &params); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		result, err := winrmRunExec(params)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := providerapi.WriteRuntimeResult(result); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		_ = providerapi.WriteErrorResponse(req, "unsupported_method", fmt.Sprintf("unsupported method %q", req.Method))
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

func winrmHealthCheck(config map[string]interface{}) (providerapi.HealthCheckResult, error) {
	if providerapi.String(config, "addr") != "" {
		if _, err := winrmlib.ConfigFromMaps(config, nil); err != nil {
			return providerapi.HealthCheckResult{}, err
		}
	}
	return providerapi.HealthCheckResult{
		OK:      true,
		Message: "winrm connector configuration looks valid",
	}, nil
}

func winrmDescribe(params providerapi.ConnectorDescribeParams) (providerapi.ConnectorDescribeResult, error) {
	addr := providerapi.String(params.Target.Config, "addr")
	if addr == "" {
		addr = providerapi.String(params.Config, "addr")
	}
	execSupported := addr != ""
	capabilities := map[string]providerapi.ConnectorCapability{
		"shell": {
			Supported: false,
			Reason:    "interactive shell is not supported by the winrm connector",
		},
		"exec": {
			Supported: execSupported,
			Reason:    unsupportedReason(execSupported, "target addr is required for winrm exec"),
			Requires:  []string{"winrm:credentials"},
			Preferred: true,
		},
		"exec_pty": {
			Supported: false,
			Reason:    "winrm does not provide SSH-style PTY execution in the first implementation wave",
		},
		"upload": {
			Supported: false,
			Reason:    "winrm file transfer is deferred to a later implementation phase",
		},
		"download": {
			Supported: false,
			Reason:    "winrm file transfer is deferred to a later implementation phase",
		},
		"port_forward_local": {
			Supported: false,
			Reason:    "winrm does not provide native local port forwarding",
		},
		"port_forward_remote": {
			Supported: false,
			Reason:    "winrm does not provide native remote port forwarding",
		},
		"mount": {
			Supported: false,
			Reason:    "winrm does not provide mount semantics",
		},
		"agent_forward": {
			Supported: false,
			Reason:    "winrm does not provide agent forwarding",
		},
	}
	return providerapi.ConnectorDescribeResult{Capabilities: capabilities}, nil
}

func winrmPrepare(params providerapi.ConnectorPrepareParams) (providerapi.ConnectorPrepareResult, error) {
	cfg, err := winrmlib.ConfigFromMaps(params.Config, params.Target.Config)
	if err != nil {
		return providerapi.ConnectorPrepareResult{}, err
	}

	switch params.Operation.Name {
	case "exec":
		return providerapi.ConnectorPrepareResult{
			Supported: true,
			Plan: providerapi.ConnectorPlan{
				Kind: "provider-managed",
				Details: map[string]interface{}{
					"connector":             "winrm",
					"addr":                  cfg.Host,
					"port":                  cfg.Port,
					"user":                  cfg.User,
					"pass":                  cfg.Password,
					"https":                 cfg.HTTPS,
					"insecure":              cfg.Insecure,
					"transport":             winrmTransport(params.Config, params.Target),
					"command":               params.Operation.Command,
					"command_line":          strings.Join(params.Operation.Command, " "),
					"shell_command":         cfg.ShellCommand,
					"tls_server_name":       cfg.TLSServerName,
					"operation_timeout_sec": cfg.OperationTimeoutSec,
				},
			},
		}, nil
	case "shell":
		return providerapi.ConnectorPrepareResult{
			Supported: false,
			Plan: providerapi.ConnectorPlan{
				Kind: "provider-managed",
				Details: map[string]interface{}{
					"connector":             "winrm",
					"addr":                  cfg.Host,
					"port":                  cfg.Port,
					"user":                  cfg.User,
					"pass":                  cfg.Password,
					"https":                 cfg.HTTPS,
					"insecure":              cfg.Insecure,
					"transport":             winrmTransport(params.Config, params.Target),
					"reason":                "interactive shell is not supported by the winrm connector",
					"shell_command":         cfg.ShellCommand,
					"tls_server_name":       cfg.TLSServerName,
					"operation_timeout_sec": cfg.OperationTimeoutSec,
				},
			},
		}, nil
	default:
		return providerapi.ConnectorPrepareResult{
			Supported: false,
			Plan: providerapi.ConnectorPlan{
				Kind: "provider-managed",
				Details: map[string]interface{}{
					"connector": "winrm",
					"reason":    fmt.Sprintf("operation %q is not supported by the winrm connector", params.Operation.Name),
				},
			},
		}, nil
	}
}

func winrmTransport(config map[string]interface{}, target providerapi.ConnectorTarget) string {
	if transport := providerapi.String(target.Config, "transport"); transport != "" {
		return strings.ToLower(transport)
	}
	if transport := providerapi.String(config, "transport"); transport != "" {
		return strings.ToLower(transport)
	}
	if winrmBool(target.Config, "insecure") || winrmBool(config, "insecure") {
		return "http"
	}
	return "https"
}

func winrmBool(raw map[string]interface{}, key string) bool {
	switch strings.ToLower(providerapi.String(raw, key)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func unsupportedReason(supported bool, reason string) string {
	if supported {
		return ""
	}
	return reason
}
