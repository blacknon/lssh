package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/blacknon/lssh/internal/providerapi"
	"github.com/blacknon/lssh/internal/providerbuiltin"
	"github.com/blacknon/lssh/provider/connector/provider-connector-openssh/opensshlib"
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
			Name:            "provider-connector-openssh",
			Capabilities:    []string{"connector"},
			Methods:         []string{providerapi.MethodPluginDescribe, providerapi.MethodHealthCheck, providerapi.MethodConnectorDescribe, providerapi.MethodConnectorPrepare},
			ProtocolVersion: providerapi.Version,
		}, nil)
	case providerapi.MethodHealthCheck:
		var params providerapi.HealthCheckParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := opensshHealthCheck(params.Config)
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
		result, err := opensshDescribe(params)
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
		result, err := opensshPrepare(params)
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

func opensshHealthCheck(config map[string]interface{}) (providerapi.HealthCheckResult, error) {
	path := providerbuiltin.String(config, "ssh_path")
	if path == "" {
		path = "ssh"
	}
	if _, err := exec.LookPath(path); err != nil {
		return providerapi.HealthCheckResult{}, err
	}
	return providerapi.HealthCheckResult{
		OK:      true,
		Message: "openssh connector can locate ssh executable",
	}, nil
}

func opensshDescribe(params providerapi.ConnectorDescribeParams) (providerapi.ConnectorDescribeResult, error) {
	_, err := opensshlib.ConfigFromMaps(params.Config, params.Target.Config)
	supported := err == nil
	reason := unsupportedReason(supported, "target addr is required for openssh transport")
	mountSupported := supported && (runtime.GOOS == "linux" || runtime.GOOS == "darwin")
	mountReason := unsupportedReason(mountSupported, "mount via sftp_transport currently supports linux and macos only")
	if !supported {
		mountReason = reason
	}

	return providerapi.ConnectorDescribeResult{
		Capabilities: map[string]providerapi.ConnectorCapability{
			"shell":                  {Supported: supported, Reason: reason, Preferred: true},
			"exec":                   {Supported: supported, Reason: reason, Preferred: true},
			"exec_pty":               {Supported: supported, Reason: reason},
			"shell_transport":        {Supported: supported, Reason: reason},
			"exec_transport":         {Supported: supported, Reason: reason},
			"sftp_transport":         {Supported: supported, Reason: reason},
			"port_forward_transport": {Supported: supported, Reason: reason},
			"upload": {
				Supported: supported,
				Reason:    reason,
				Requires:  []string{"sftp_transport"},
			},
			"download": {
				Supported: supported,
				Reason:    reason,
				Requires:  []string{"sftp_transport"},
			},
			"port_forward_local": {
				Supported: supported,
				Reason:    reason,
				Requires:  []string{"port_forward_transport"},
			},
			"port_forward_remote": {
				Supported: supported,
				Reason:    reason,
				Requires:  []string{"port_forward_transport"},
			},
			"agent_forward": {
				Supported: supported,
				Reason:    reason,
			},
			"mount": {
				Supported: mountSupported,
				Reason:    mountReason,
				Requires:  []string{"sftp_transport"},
			},
		},
	}, nil
}

func opensshPrepare(params providerapi.ConnectorPrepareParams) (providerapi.ConnectorPrepareResult, error) {
	cfg, err := opensshlib.ConfigFromMaps(params.Config, params.Target.Config)
	if err != nil {
		return providerapi.ConnectorPrepareResult{}, err
	}

	switch params.Operation.Name {
	case "shell":
		return providerapi.ConnectorPrepareResult{
			Supported: true,
			Plan:      opensshlib.BuildShellPlan(cfg),
		}, nil
	case "exec", "exec_pty":
		return providerapi.ConnectorPrepareResult{
			Supported: true,
			Plan:      opensshlib.BuildExecPlan(cfg, params.Operation),
		}, nil
	case "sftp_transport", "upload", "download":
		return providerapi.ConnectorPrepareResult{
			Supported: true,
			Plan:      opensshlib.BuildSFTPTransportPlan(cfg),
		}, nil
	case "mount":
		if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
			return providerapi.ConnectorPrepareResult{
				Supported: false,
				Plan: providerapi.ConnectorPlan{
					Kind: "command",
					Details: map[string]interface{}{
						"connector": "openssh",
						"reason":    "mount via sftp_transport currently supports linux and macos only",
					},
				},
			}, nil
		}
		return providerapi.ConnectorPrepareResult{
			Supported: true,
			Plan:      opensshlib.BuildSFTPTransportPlan(cfg),
		}, nil
	default:
		return providerapi.ConnectorPrepareResult{
			Supported: false,
			Plan: providerapi.ConnectorPlan{
				Kind: "command",
				Details: map[string]interface{}{
					"connector": "openssh",
					"reason":    fmt.Sprintf("operation %q is not supported by the openssh connector", params.Operation.Name),
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
