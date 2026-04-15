package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	onepassword "github.com/1password/onepassword-sdk-go"
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
			Name:            "provider-secret-onepassword",
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

		value, err := getSecret(params)
		if err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "secret_get_failed", err.Error())
			os.Exit(1)
		}
		_ = providerbuiltin.WriteResponse(req, providerapi.SecretGetResult{Value: value}, nil)
	case providerapi.MethodHealthCheck:
		var params providerapi.HealthCheckParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := onePasswordHealthCheck(params.Config)
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

func getSecret(params providerapi.SecretGetParams) (string, error) {
	token, err := providerbuiltin.ResolveConfigValue(params.Config, "token")
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", fmt.Errorf("provider.onepassword.token is required for SDK authentication")
	}

	client, err := onepassword.NewClient(
		context.Background(),
		onepassword.WithServiceAccountToken(token),
		onepassword.WithIntegrationInfo("lssh", "provider-secret-onepassword"),
	)
	if err != nil {
		return "", err
	}

	return client.Secrets().Resolve(context.Background(), params.Ref)
}

func onePasswordHealthCheck(config map[string]interface{}) (providerapi.HealthCheckResult, error) {
	token, err := providerbuiltin.ResolveConfigValue(config, "token")
	if err != nil {
		return providerapi.HealthCheckResult{}, err
	}
	if token == "" {
		return providerapi.HealthCheckResult{}, fmt.Errorf("provider.onepassword.token is required for SDK authentication")
	}

	_, err = onepassword.NewClient(
		context.Background(),
		onepassword.WithServiceAccountToken(token),
		onepassword.WithIntegrationInfo("lssh", "provider-secret-onepassword"),
	)
	if err != nil {
		return providerapi.HealthCheckResult{}, err
	}

	return providerapi.HealthCheckResult{
		OK:      true,
		Message: "onepassword secret provider initialized successfully",
	}, nil
}
