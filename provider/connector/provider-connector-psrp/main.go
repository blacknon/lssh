package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blacknon/lssh/providerapi"
	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib"
)

func main() {
	if psrplib.IsHelperInvocation(os.Args[1:]) {
		os.Exit(psrplib.RunHelper(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
	}

	req, err := providerapi.ReadRequest()
	if err != nil {
		_ = providerapi.WriteError(err.Error())
		os.Exit(1)
	}

	switch req.Method {
	case providerapi.MethodPluginDescribe:
		_ = providerapi.WriteResponse(req, providerapi.PluginDescribeResult{
			Name:            "provider-connector-psrp",
			Capabilities:    []string{"connector"},
			ConnectorNames:  []string{"psrp"},
			Methods:         []string{providerapi.MethodPluginDescribe, providerapi.MethodHealthCheck, providerapi.MethodConnectorDescribe, providerapi.MethodConnectorPrepare},
			ProtocolVersion: providerapi.Version,
		}, nil)
	case providerapi.MethodHealthCheck:
		var params providerapi.HealthCheckParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := psrpHealthCheck(params.Config)
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
		result, err := psrpDescribe(params)
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
		result, err := psrpPrepare(params)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "connector_prepare_failed", err.Error())
			os.Exit(1)
		}
		_ = providerapi.WriteResponse(req, result, nil)
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

func psrpHealthCheck(config map[string]interface{}) (providerapi.HealthCheckResult, error) {
	runtime := psrplib.ResolveRuntime(config, nil)

	if runtime.Shell == psrplib.RuntimeCommand || runtime.Exec == psrplib.RuntimeCommand {
		program, err := psrplib.ResolvePowerShellPath(providerapi.String(config, "pwsh_path"))
		if err != nil {
			return providerapi.HealthCheckResult{}, err
		}
		return providerapi.HealthCheckResult{
			OK:      true,
			Message: fmt.Sprintf("psrp connector can use %s", program),
		}, nil
	}

	return providerapi.HealthCheckResult{
		OK:      true,
		Message: "psrp connector configuration looks valid",
	}, nil
}

func psrpDescribe(params providerapi.ConnectorDescribeParams) (providerapi.ConnectorDescribeResult, error) {
	addr := providerapi.String(params.Target.Config, "addr")
	if addr == "" {
		addr = providerapi.String(params.Config, "addr")
	}
	runtime := psrplib.ResolveRuntime(params.Config, params.Target.Config)

	var shellReason string
	var execReason string
	shellSupported := addr != "" && psrpInteractiveShellEnabled(params.Config, params.Target)
	execSupported := addr != ""

	if !psrplib.RuntimeSupported(runtime.Shell) {
		shellSupported = false
		shellReason = psrplib.RuntimeUnsupportedReason(runtime.Shell)
	} else if runtime.Shell == psrplib.RuntimeCommand {
		_, pathErr := psrplib.ResolvePowerShellPath(providerapi.String(params.Target.Config, "pwsh_path"))
		if pathErr != nil {
			_, pathErr = psrplib.ResolvePowerShellPath(providerapi.String(params.Config, "pwsh_path"))
		}
		if pathErr != nil {
			shellSupported = false
			shellReason = pathErr.Error()
		}
	}

	if !psrplib.RuntimeSupported(runtime.Exec) {
		execSupported = false
		execReason = psrplib.RuntimeUnsupportedReason(runtime.Exec)
	} else if runtime.Exec == psrplib.RuntimeCommand {
		_, pathErr := psrplib.ResolvePowerShellPath(providerapi.String(params.Target.Config, "pwsh_path"))
		if pathErr != nil {
			_, pathErr = psrplib.ResolvePowerShellPath(providerapi.String(params.Config, "pwsh_path"))
		}
		if pathErr != nil {
			execSupported = false
			execReason = pathErr.Error()
		}
	}

	capabilities := map[string]providerapi.ConnectorCapability{
		"shell": {
			Supported: shellSupported,
			Reason:    unsupportedReason(shellSupported, describeReason(addr != "", shellReason, "interactive shell support is disabled unless enable_shell=true")),
			Requires:  shellRequires(runtime.Shell),
		},
		"exec": {
			Supported: execSupported,
			Reason:    unsupportedReason(execSupported, describeReason(addr != "", execReason, "target addr is required for psrp exec")),
			Requires:  execRequires(runtime.Exec),
			Preferred: true,
		},
		"exec_pty": {
			Supported: false,
			Reason:    "psrp exec uses PowerShell remoting pipelines and does not expose SSH-style PTY execution",
		},
		"upload": {
			Supported: false,
			Reason:    "file transfer is deferred to a later psrp implementation phase",
		},
		"download": {
			Supported: false,
			Reason:    "file transfer is deferred to a later psrp implementation phase",
		},
		"port_forward_local": {
			Supported: false,
			Reason:    "psrp does not provide native local port forwarding",
		},
		"port_forward_remote": {
			Supported: false,
			Reason:    "psrp does not provide native remote port forwarding",
		},
		"mount": {
			Supported: false,
			Reason:    "psrp does not provide mount semantics",
		},
		"agent_forward": {
			Supported: false,
			Reason:    "psrp does not provide agent forwarding",
		},
	}

	return providerapi.ConnectorDescribeResult{Capabilities: capabilities}, nil
}

func psrpPrepare(params providerapi.ConnectorPrepareParams) (providerapi.ConnectorPrepareResult, error) {
	cfg, err := psrplib.ConfigFromMaps(params.Config, params.Target.Config)
	if err != nil {
		return providerapi.ConnectorPrepareResult{}, err
	}
	runtime := psrplib.ResolveRuntime(params.Config, params.Target.Config)

	switch params.Operation.Name {
	case "shell":
		supported := psrpInteractiveShellEnabled(params.Config, params.Target)
		if !supported {
			return providerapi.ConnectorPrepareResult{
				Supported: false,
				Plan: providerapi.ConnectorPlan{
					Kind: "command",
					Details: map[string]interface{}{
						"connector": "psrp",
						"reason":    "interactive shell support is disabled unless enable_shell=true",
					},
				},
			}, nil
		}
		plan, err := psrplib.BuildPlan(cfg, runtime.Shell, params.Operation)
		if err != nil {
			return providerapi.ConnectorPrepareResult{}, err
		}
		return providerapi.ConnectorPrepareResult{
			Supported: true,
			Plan:      plan,
		}, nil
	case "exec":
		plan, err := psrplib.BuildPlan(cfg, runtime.Exec, params.Operation)
		if err != nil {
			return providerapi.ConnectorPrepareResult{}, err
		}
		return providerapi.ConnectorPrepareResult{
			Supported: true,
			Plan:      plan,
		}, nil
	default:
		return providerapi.ConnectorPrepareResult{
			Supported: false,
			Plan: providerapi.ConnectorPlan{
				Kind: "command",
				Details: map[string]interface{}{
					"connector": "psrp",
					"reason":    fmt.Sprintf("operation %q is not supported by the psrp connector", params.Operation.Name),
				},
			},
		}, nil
	}
}

func psrpInteractiveShellEnabled(config map[string]interface{}, target providerapi.ConnectorTarget) bool {
	if psrpBool(target.Config, "enable_shell") {
		return true
	}
	return psrpBool(config, "enable_shell")
}

func psrpBool(raw map[string]interface{}, key string) bool {
	switch providerapi.String(raw, key) {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
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

func describeReason(hasAddr bool, reason string, fallback string) string {
	switch {
	case !hasAddr:
		return "target addr is required"
	case reason != "":
		return reason
	default:
		return fallback
	}
}

func shellRequires(runtime string) []string {
	switch psrplib.ResolveRuntime(map[string]interface{}{"runtime": runtime}, nil).Shell {
	case psrplib.RuntimeCommand:
		return []string{"psrp:credentials", "pwsh:wsman"}
	default:
		return []string{"psrp:credentials"}
	}
}

func execRequires(runtime string) []string {
	switch psrplib.ResolveRuntime(map[string]interface{}{"runtime": runtime}, nil).Exec {
	case psrplib.RuntimeCommand:
		return []string{"psrp:credentials", "pwsh:wsman"}
	default:
		return []string{"psrp:credentials"}
	}
}
