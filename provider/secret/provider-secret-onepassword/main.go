package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	onepassword "github.com/1password/onepassword-sdk-go"
	"github.com/blacknon/lssh/providerapi"
)

const (
	onePasswordAuthModeAuto           = "auto"
	onePasswordAuthModeServiceAccount = "service_account"
	onePasswordAuthModeCLI            = "cli"
)

var runOnePasswordCLI = func(config map[string]interface{}, args ...string) ([]byte, error) {
	return providerapi.Run(onePasswordCLIPath(config), args...)
}

func main() {
	req, err := providerapi.ReadRequest()
	if err != nil {
		_ = providerapi.WriteError(err.Error())
		os.Exit(1)
	}

	switch req.Method {
	case providerapi.MethodPluginDescribe:
		_ = providerapi.WriteResponse(req, providerapi.PluginDescribeResult{
			Name:            "provider-secret-onepassword",
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

		value, err := getSecret(params)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, onePasswordProviderErrorCode(err, "secret_get_failed"), err.Error())
			os.Exit(1)
		}
		_ = providerapi.WriteResponse(req, providerapi.SecretGetResult{Value: value}, nil)
	case providerapi.MethodHealthCheck:
		var params providerapi.HealthCheckParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := onePasswordHealthCheck(params.Config)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, onePasswordProviderErrorCode(err, "health_check_failed"), err.Error())
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

func getSecret(params providerapi.SecretGetParams) (string, error) {
	mode, err := onePasswordAuthMode(params.Config)
	if err != nil {
		return "", err
	}

	switch mode {
	case onePasswordAuthModeServiceAccount:
		token, err := onePasswordToken(params.Config)
		if err != nil {
			return "", err
		}
		return getSecretWithSDK(token, params.Ref)
	case onePasswordAuthModeCLI:
		return getSecretWithCLI(params.Config, params.Ref)
	case onePasswordAuthModeAuto:
		token, err := providerapi.ResolveConfigValue(params.Config, "token")
		if err != nil {
			return "", err
		}
		if token != "" {
			return getSecretWithSDK(token, params.Ref)
		}
		return getSecretWithCLI(params.Config, params.Ref)
	default:
		return "", fmt.Errorf("unsupported auth_mode %q", mode)
	}
}

func getSecretWithSDK(token, ref string) (string, error) {
	client, err := onepassword.NewClient(
		context.Background(),
		onepassword.WithServiceAccountToken(token),
		onepassword.WithIntegrationInfo("lssh", "provider-secret-onepassword"),
	)
	if err != nil {
		return "", err
	}

	return client.Secrets().Resolve(context.Background(), ref)
}

func onePasswordHealthCheck(config map[string]interface{}) (providerapi.HealthCheckResult, error) {
	mode, err := onePasswordAuthMode(config)
	if err != nil {
		return providerapi.HealthCheckResult{}, err
	}

	switch mode {
	case onePasswordAuthModeServiceAccount:
		token, err := onePasswordToken(config)
		if err != nil {
			return providerapi.HealthCheckResult{}, err
		}
		if err := onePasswordSDKHealthCheck(token); err != nil {
			return providerapi.HealthCheckResult{}, err
		}
		return providerapi.HealthCheckResult{
			OK:      true,
			Message: "onepassword secret provider initialized successfully with service account auth",
		}, nil
	case onePasswordAuthModeCLI:
		if err := onePasswordCLIHealthCheck(config); err != nil {
			return providerapi.HealthCheckResult{}, err
		}
		return providerapi.HealthCheckResult{
			OK:      true,
			Message: "onepassword secret provider can use the op CLI session",
		}, nil
	case onePasswordAuthModeAuto:
		token, err := providerapi.ResolveConfigValue(config, "token")
		if err != nil {
			return providerapi.HealthCheckResult{}, err
		}
		if token != "" {
			if err := onePasswordSDKHealthCheck(token); err != nil {
				return providerapi.HealthCheckResult{}, err
			}
			return providerapi.HealthCheckResult{
				OK:      true,
				Message: "onepassword secret provider initialized successfully with service account auth",
			}, nil
		}
		if err := onePasswordCLIHealthCheck(config); err != nil {
			return providerapi.HealthCheckResult{}, err
		}
		return providerapi.HealthCheckResult{
			OK:      true,
			Message: "onepassword secret provider can use the op CLI session",
		}, nil
	default:
		return providerapi.HealthCheckResult{}, fmt.Errorf("unsupported auth_mode %q", mode)
	}
}

func onePasswordSDKHealthCheck(token string) error {
	_, err := onepassword.NewClient(
		context.Background(),
		onepassword.WithServiceAccountToken(token),
		onepassword.WithIntegrationInfo("lssh", "provider-secret-onepassword"),
	)
	if err != nil {
		return err
	}

	return nil
}

func onePasswordCLIHealthCheck(config map[string]interface{}) error {
	_, err := runOnePasswordCLI(config, "whoami")
	return err
}

func getSecretWithCLI(config map[string]interface{}, ref string) (string, error) {
	output, err := runOnePasswordCLI(config, "read", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(output), "\n"), nil
}

func onePasswordToken(config map[string]interface{}) (string, error) {
	token, err := providerapi.ResolveConfigValue(config, "token")
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", fmt.Errorf("provider.onepassword.token is required for service account authentication")
	}
	return token, nil
}

func onePasswordAuthMode(config map[string]interface{}) (string, error) {
	mode := strings.ToLower(providerapi.String(config, "auth_mode"))
	if mode == "" {
		return onePasswordAuthModeAuto, nil
	}
	switch mode {
	case onePasswordAuthModeAuto, onePasswordAuthModeServiceAccount, onePasswordAuthModeCLI:
		return mode, nil
	default:
		return "", fmt.Errorf("provider.onepassword.auth_mode must be one of auto, service_account, cli")
	}
}

func onePasswordCLIPath(config map[string]interface{}) string {
	if path := providerapi.String(config, "op_path"); path != "" {
		return path
	}
	return "op"
}

func onePasswordProviderErrorCode(err error, fallback string) string {
	if onePasswordCLIAuthPending(err) {
		return "auth_pending"
	}
	return fallback
}

func onePasswordCLIAuthPending(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	patterns := []string{
		"touch id",
		"biometric",
		"authentication required",
		"authorization required",
		"sign in to your 1password account",
		"you are not currently signed in",
		"unlock 1password",
		"1password app",
	}
	for _, pattern := range patterns {
		if strings.Contains(message, pattern) {
			return true
		}
	}

	return false
}
