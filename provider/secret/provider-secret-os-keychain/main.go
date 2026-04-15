package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/blacknon/lssh/internal/providerapi"
	"github.com/blacknon/lssh/internal/providerbuiltin"
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
			Name:            "provider-secret-os-keychain",
			Capabilities:    []string{"secret"},
			Methods:         []string{providerapi.MethodPluginDescribe, providerapi.MethodHealthCheck, providerapi.MethodSecretGet},
			ProtocolVersion: providerapi.Version,
		}, nil)
	case providerapi.MethodSecretGet:
		var params providerapi.SecretGetParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}

		service, account, err := parseKeychainRef(params.Ref)
		if err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "invalid_ref", err.Error())
			os.Exit(1)
		}

		output, err := providerbuiltin.Run("security", "find-generic-password", "-s", service, "-a", account, "-w")
		if err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "secret_get_failed", err.Error())
			os.Exit(1)
		}
		_ = providerbuiltin.WriteResponse(req, providerapi.SecretGetResult{Value: strings.TrimSpace(string(output)), Type: "password"}, nil)
	case providerapi.MethodHealthCheck:
		result, err := keychainHealthCheck()
		if err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "health_check_failed", err.Error())
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

func parseKeychainRef(ref string) (string, string, error) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("keychain ref must be service/account")
	}
	return parts[0], parts[1], nil
}

func keychainHealthCheck() (providerapi.HealthCheckResult, error) {
	if runtime.GOOS != "darwin" {
		return providerapi.HealthCheckResult{}, fmt.Errorf("os-keychain provider is only supported on darwin")
	}
	if _, err := exec.LookPath("security"); err != nil {
		return providerapi.HealthCheckResult{}, fmt.Errorf("security command not found: %w", err)
	}
	return providerapi.HealthCheckResult{
		OK:      true,
		Message: "os-keychain secret provider can access the security command",
	}, nil
}
