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

const (
	bitwardenAuthModeAuto = "auto"
	bitwardenAuthModeSDK  = "sdk"
	bitwardenAuthModeCLI  = "cli"
)

type bitwardenCLIItem struct {
	Name   string `json:"name"`
	Key    string `json:"key"`
	Notes  string `json:"notes"`
	Fields []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"fields"`
	SSHKey *struct {
		PrivateKey     string `json:"privateKey"`
		PublicKey      string `json:"publicKey"`
		KeyFingerprint string `json:"keyFingerprint"`
	} `json:"sshKey"`
	Login *struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Uris     []struct {
			URI string `json:"uri"`
		} `json:"uris"`
	} `json:"login"`
}

var runBitwardenCLI = func(config map[string]interface{}, args ...string) ([]byte, error) {
	commandArgs := make([]string, 0, len(args)+3)
	commandArgs = append(commandArgs, args...)
	commandArgs = append(commandArgs, "--nointeraction")
	if session, err := providerbuiltin.ResolveConfigValue(config, "session"); err == nil && session != "" {
		commandArgs = append(commandArgs, "--session", session)
	}
	return providerbuiltin.RunWithEnv(bitwardenCLIEnv(config), bitwardenCLIPath(config), commandArgs...)
}

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

		value, resultType, err := getSecret(params)
		if err != nil {
			_ = providerbuiltin.WriteErrorResponse(req, "secret_get_failed", err.Error())
			os.Exit(1)
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

func getSecret(params providerapi.SecretGetParams) (string, string, error) {
	mode, err := bitwardenAuthMode(params.Config)
	if err != nil {
		return "", "", err
	}

	switch mode {
	case bitwardenAuthModeSDK:
		accessToken, err := bitwardenToken(params.Config)
		if err != nil {
			return "", "", err
		}
		return getSecretWithSDK(params.Config, accessToken, params.Ref)
	case bitwardenAuthModeCLI:
		return getSecretWithCLI(params.Config, params.Ref)
	case bitwardenAuthModeAuto:
		accessToken, err := providerbuiltin.ResolveConfigValue(params.Config, "token")
		if err != nil {
			return "", "", err
		}
		if accessToken != "" {
			return getSecretWithSDK(params.Config, accessToken, params.Ref)
		}
		return getSecretWithCLI(params.Config, params.Ref)
	default:
		return "", "", fmt.Errorf("unsupported auth_mode %q", mode)
	}
}

func getSecretWithSDK(config map[string]interface{}, accessToken, ref string) (string, string, error) {
	secretID, field := splitBitwardenRef(ref)

	apiURL, identityURL := bitwardenEndpoints(config)
	client, err := sdk.NewBitwardenClient(apiURL, identityURL)
	if err != nil {
		return "", "", err
	}
	defer client.Close()

	if err := client.AccessTokenLogin(accessToken, nil); err != nil {
		return "", "", err
	}

	secret, err := client.Secrets().Get(secretID)
	if err != nil {
		return "", "", err
	}

	switch field {
	case "", "value", "password":
		return secret.Value, "password", nil
	case "note", "notes":
		return secret.Note, "text", nil
	case "key":
		return secret.Key, "text", nil
	default:
		return "", "", fmt.Errorf("bitwarden field %q is not supported by the SDK provider", field)
	}
}

func getSecretWithCLI(config map[string]interface{}, ref string) (string, string, error) {
	itemID, field := splitBitwardenRef(ref)
	if field == "" {
		field = "value"
	}

	switch field {
	case "value", "password":
		output, err := runBitwardenCLI(config, "get", "password", itemID)
		if err != nil {
			return "", "", err
		}
		return strings.TrimRight(string(output), "\n"), "password", nil
	case "username":
		output, err := runBitwardenCLI(config, "get", "username", itemID)
		if err != nil {
			return "", "", err
		}
		return strings.TrimRight(string(output), "\n"), "text", nil
	case "totp":
		output, err := runBitwardenCLI(config, "get", "totp", itemID)
		if err != nil {
			return "", "", err
		}
		return strings.TrimRight(string(output), "\n"), "text", nil
	case "note", "notes", "key", "public_key", "fingerprint", "uri":
		item, err := getBitwardenCLIItem(config, itemID)
		if err != nil {
			return "", "", err
		}

		switch field {
		case "note", "notes":
			return item.Notes, "text", nil
		case "key":
			if item.SSHKey != nil && item.SSHKey.PrivateKey != "" {
				return item.SSHKey.PrivateKey, "text", nil
			}
			if item.Key != "" {
				return item.Key, "text", nil
			}
			if value, ok := bitwardenCLICustomField(item, "key"); ok {
				return value, "text", nil
			}
			if item.Notes != "" {
				return item.Notes, "text", nil
			}
			return "", "", fmt.Errorf("bitwarden item %q does not have sshKey.privateKey, custom field %q, or notes", itemID, "key")
		case "public_key":
			if item.SSHKey != nil && item.SSHKey.PublicKey != "" {
				return item.SSHKey.PublicKey, "text", nil
			}
			return "", "", fmt.Errorf("bitwarden item %q does not have sshKey.publicKey", itemID)
		case "fingerprint":
			if item.SSHKey != nil && item.SSHKey.KeyFingerprint != "" {
				return item.SSHKey.KeyFingerprint, "text", nil
			}
			return "", "", fmt.Errorf("bitwarden item %q does not have sshKey.keyFingerprint", itemID)
		case "uri":
			if item.Login != nil && len(item.Login.Uris) > 0 {
				return item.Login.Uris[0].URI, "text", nil
			}
			return "", "", fmt.Errorf("bitwarden item %q does not have a uri", itemID)
		}
	default:
		if strings.HasPrefix(field, "field:") {
			item, err := getBitwardenCLIItem(config, itemID)
			if err != nil {
				return "", "", err
			}

			fieldName := strings.TrimPrefix(field, "field:")
			if fieldName == "" {
				return "", "", fmt.Errorf("bitwarden custom field name must not be empty")
			}

			value, ok := bitwardenCLICustomField(item, fieldName)
			if !ok {
				return "", "", fmt.Errorf("bitwarden item %q does not have custom field %q", itemID, fieldName)
			}
			return value, "text", nil
		}
		return "", "", fmt.Errorf("bitwarden field %q is not supported by the CLI provider", field)
	}

	return "", "", fmt.Errorf("bitwarden field %q is not supported by the CLI provider", field)
}

func getBitwardenCLIItem(config map[string]interface{}, itemID string) (bitwardenCLIItem, error) {
	output, err := runBitwardenCLI(config, "get", "item", itemID)
	if err != nil {
		return bitwardenCLIItem{}, err
	}

	var item bitwardenCLIItem
	if err := json.Unmarshal(output, &item); err != nil {
		return bitwardenCLIItem{}, err
	}
	return item, nil
}

func bitwardenCLICustomField(item bitwardenCLIItem, name string) (string, bool) {
	for _, field := range item.Fields {
		if strings.EqualFold(field.Name, name) {
			return field.Value, true
		}
	}
	return "", false
}

func splitBitwardenRef(ref string) (string, string) {
	idx := strings.LastIndex(ref, "/")
	if idx == -1 {
		return ref, "value"
	}

	locator := ref[:idx]
	field := ref[idx+1:]
	if locator == "" || field == "" {
		return ref, "value"
	}

	if bitwardenSupportedField(field) || strings.HasPrefix(field, "field:") {
		return locator, field
	}

	return ref, "value"
}

func bitwardenSupportedField(field string) bool {
	switch field {
	case "value", "password", "username", "uri", "totp", "note", "notes", "key", "public_key", "fingerprint":
		return true
	default:
		return false
	}
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
	mode, err := bitwardenAuthMode(config)
	if err != nil {
		return providerapi.HealthCheckResult{}, err
	}

	switch mode {
	case bitwardenAuthModeSDK:
		accessToken, err := bitwardenToken(config)
		if err != nil {
			return providerapi.HealthCheckResult{}, err
		}
		if err := bitwardenSDKHealthCheck(config, accessToken); err != nil {
			return providerapi.HealthCheckResult{}, err
		}
		return providerapi.HealthCheckResult{
			OK:      true,
			Message: "bitwarden secret provider authenticated successfully with SDK access token auth",
		}, nil
	case bitwardenAuthModeCLI:
		if err := bitwardenCLIHealthCheck(config); err != nil {
			return providerapi.HealthCheckResult{}, err
		}
		return providerapi.HealthCheckResult{
			OK:      true,
			Message: "bitwarden secret provider can use the bw CLI session",
		}, nil
	case bitwardenAuthModeAuto:
		accessToken, err := providerbuiltin.ResolveConfigValue(config, "token")
		if err != nil {
			return providerapi.HealthCheckResult{}, err
		}
		if accessToken != "" {
			if err := bitwardenSDKHealthCheck(config, accessToken); err != nil {
				return providerapi.HealthCheckResult{}, err
			}
			return providerapi.HealthCheckResult{
				OK:      true,
				Message: "bitwarden secret provider authenticated successfully with SDK access token auth",
			}, nil
		}
		if err := bitwardenCLIHealthCheck(config); err != nil {
			return providerapi.HealthCheckResult{}, err
		}
		return providerapi.HealthCheckResult{
			OK:      true,
			Message: "bitwarden secret provider can use the bw CLI session",
		}, nil
	default:
		return providerapi.HealthCheckResult{}, fmt.Errorf("unsupported auth_mode %q", mode)
	}
}

func bitwardenSDKHealthCheck(config map[string]interface{}, accessToken string) error {
	apiURL, identityURL := bitwardenEndpoints(config)
	client, err := sdk.NewBitwardenClient(apiURL, identityURL)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.AccessTokenLogin(accessToken, nil)
}

func bitwardenCLIHealthCheck(config map[string]interface{}) error {
	_, err := runBitwardenCLI(config, "status")
	return err
}

func bitwardenToken(config map[string]interface{}) (string, error) {
	accessToken, err := providerbuiltin.ResolveConfigValue(config, "token")
	if err != nil {
		return "", err
	}
	if accessToken == "" {
		return "", fmt.Errorf("provider.bitwarden.token is required for SDK authentication")
	}
	return accessToken, nil
}

func bitwardenAuthMode(config map[string]interface{}) (string, error) {
	mode := strings.ToLower(providerbuiltin.String(config, "auth_mode"))
	if mode == "" {
		return bitwardenAuthModeAuto, nil
	}
	switch mode {
	case bitwardenAuthModeAuto, bitwardenAuthModeSDK, bitwardenAuthModeCLI:
		return mode, nil
	default:
		return "", fmt.Errorf("provider.bitwarden.auth_mode must be one of auto, sdk, cli")
	}
}

func bitwardenCLIPath(config map[string]interface{}) string {
	if path := providerbuiltin.String(config, "bw_path"); path != "" {
		return path
	}
	return "bw"
}

func bitwardenCLIEnv(config map[string]interface{}) []string {
	env := []string{}

	if appdataDir, err := providerbuiltin.ResolveConfigValue(config, "appdata_dir"); err == nil && appdataDir != "" {
		env = append(env, "BITWARDENCLI_APPDATA_DIR="+appdataDir)
	}

	return env
}
