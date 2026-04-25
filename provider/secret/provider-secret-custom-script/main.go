package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
			Name:            "provider-secret-custom-script",
			Capabilities:    []string{"secret"},
			Methods:         []string{providerapi.MethodPluginDescribe, providerapi.MethodHealthCheck, providerapi.MethodSecretGet},
			ProtocolVersion: providerapi.Version,
		}, nil)
	case providerapi.MethodSecretGet:
		var params providerapi.SecretGetParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}

		command, err := customScriptCommand(params.Config)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_config", err.Error())
			os.Exit(1)
		}

		cmd := exec.Command(command[0], command[1:]...)
		cmd.Env = append(os.Environ(),
			"LSSH_PROVIDER_METHOD=secret.get",
			"LSSH_PROVIDER_REF="+params.Ref,
			"LSSH_PROVIDER_SERVER="+params.Server,
			"LSSH_PROVIDER_FIELD="+params.Field,
		)
		output, err := cmd.Output()
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "secret_get_failed", err.Error())
			os.Exit(1)
		}

		_ = providerapi.WriteResponse(req, providerapi.SecretGetResult{Value: strings.TrimRight(string(output), "\n")}, nil)
	case providerapi.MethodHealthCheck:
		var params providerapi.HealthCheckParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := customScriptHealthCheck(params.Config)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "health_check_failed", err.Error())
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

func customScriptCommand(config map[string]interface{}) ([]string, error) {
	command := providerapi.StringSlice(config, "command")
	if len(command) > 0 {
		return command, nil
	}

	if path := providerapi.String(config, "path"); path != "" {
		return []string{path}, nil
	}

	return nil, fmt.Errorf("custom-script provider requires command or path")
}

func customScriptHealthCheck(config map[string]interface{}) (providerapi.HealthCheckResult, error) {
	command, err := customScriptCommand(config)
	if err != nil {
		return providerapi.HealthCheckResult{}, err
	}

	target := command[0]
	if strings.Contains(target, string(filepath.Separator)) {
		if _, err := os.Stat(target); err != nil {
			return providerapi.HealthCheckResult{}, err
		}
	} else if _, err := exec.LookPath(target); err != nil {
		return providerapi.HealthCheckResult{}, err
	}

	return providerapi.HealthCheckResult{
		OK:      true,
		Message: "custom-script secret provider command is available",
	}, nil
}
