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
	token := providerbuiltin.String(params.Config, "token")
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
