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
	case providerapi.MethodSecretGet:
		var params providerapi.SecretGetParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteError(err.Error())
			os.Exit(1)
		}

		value, err := getSecret(params)
		if err != nil {
			_ = providerbuiltin.WriteError(err.Error())
			os.Exit(1)
		}
		_ = providerbuiltin.WriteResult(providerapi.SecretGetResult{Value: value})
	default:
		_ = providerbuiltin.WriteError(fmt.Sprintf("unsupported method %q", req.Method))
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
