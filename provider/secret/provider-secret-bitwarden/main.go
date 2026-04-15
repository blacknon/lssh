package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	sdk "github.com/bitwarden/sdk-go"
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
			Name:            "provider-secret-bitwarden",
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
		resultType := "text"
		if field := splitBitwardenField(params.Ref); field == "password" || field == "value" || field == "" {
			resultType = "password"
		}
		_ = providerbuiltin.WriteResponse(req, providerapi.SecretGetResult{Value: value, Type: resultType}, nil)
	case providerapi.MethodHealthCheck:
		var params providerapi.HealthCheckParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := bitwardenHealthCheck(params.Config)
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
	secretID, field := splitBitwardenRef(params.Ref)
	accessToken, err := providerbuiltin.ResolveConfigValue(params.Config, "token")
	if err != nil {
		return "", err
	}
	if accessToken == "" {
		accessToken, err = providerbuiltin.ResolveConfigValue(params.Config, "session")
		if err != nil {
			return "", err
		}
	}
	if accessToken == "" {
		return "", fmt.Errorf("provider.bitwarden.token is required for SDK authentication")
	}

	apiURL, identityURL := bitwardenEndpoints(params.Config)
	client, err := sdk.NewBitwardenClient(apiURL, identityURL)
	if err != nil {
		return "", err
	}
	defer client.Close()

	if err := client.AccessTokenLogin(accessToken, nil); err != nil {
		return "", err
	}

	secret, err := client.Secrets().Get(secretID)
	if err != nil {
		return "", err
	}

	switch field {
	case "", "value", "password":
		return secret.Value, nil
	case "note", "notes":
		return secret.Note, nil
	case "key":
		return secret.Key, nil
	default:
		return "", fmt.Errorf("bitwarden field %q is not supported by the SDK provider", field)
	}
}

func splitBitwardenRef(ref string) (string, string) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return ref, "value"
}

func splitBitwardenField(ref string) string {
	_, field := splitBitwardenRef(ref)
	return field
}

func bitwardenEndpoints(config map[string]interface{}) (*string, *string) {
	server, err := providerbuiltin.ResolveConfigValue(config, "server")
	if err == nil && server != "" {
		base := strings.TrimRight(server, "/")
		apiURL := base + "/api"
		identityURL := base + "/identity"
		if v, err := providerbuiltin.ResolveConfigValue(config, "api_url"); err == nil && v != "" {
			apiURL = v
		}
		if v, err := providerbuiltin.ResolveConfigValue(config, "identity_url"); err == nil && v != "" {
			identityURL = v
		}
		return &apiURL, &identityURL
	}

	var apiURL, identityURL *string
	if v, err := providerbuiltin.ResolveConfigValue(config, "api_url"); err == nil && v != "" {
		apiURL = &v
	}
	if v, err := providerbuiltin.ResolveConfigValue(config, "identity_url"); err == nil && v != "" {
		identityURL = &v
	}
	return apiURL, identityURL
}

func bitwardenHealthCheck(config map[string]interface{}) (providerapi.HealthCheckResult, error) {
	accessToken, err := providerbuiltin.ResolveConfigValue(config, "token")
	if err != nil {
		return providerapi.HealthCheckResult{}, err
	}
	if accessToken == "" {
		accessToken, err = providerbuiltin.ResolveConfigValue(config, "session")
		if err != nil {
			return providerapi.HealthCheckResult{}, err
		}
	}
	if accessToken == "" {
		return providerapi.HealthCheckResult{}, fmt.Errorf("provider.bitwarden.token is required for SDK authentication")
	}

	apiURL, identityURL := bitwardenEndpoints(config)
	client, err := sdk.NewBitwardenClient(apiURL, identityURL)
	if err != nil {
		return providerapi.HealthCheckResult{}, err
	}
	defer client.Close()

	if err := client.AccessTokenLogin(accessToken, nil); err != nil {
		return providerapi.HealthCheckResult{}, err
	}

	return providerapi.HealthCheckResult{
		OK:      true,
		Message: "bitwarden secret provider authenticated successfully",
	}, nil
}
