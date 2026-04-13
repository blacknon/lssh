package main

import (
	"encoding/json"
	"fmt"
	"os"
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
	case providerapi.MethodSecretGet:
		var params providerapi.SecretGetParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerbuiltin.WriteError(err.Error())
			os.Exit(1)
		}

		service, account, err := parseKeychainRef(params.Ref)
		if err != nil {
			_ = providerbuiltin.WriteError(err.Error())
			os.Exit(1)
		}

		output, err := providerbuiltin.Run("security", "find-generic-password", "-s", service, "-a", account, "-w")
		if err != nil {
			_ = providerbuiltin.WriteError(err.Error())
			os.Exit(1)
		}
		_ = providerbuiltin.WriteResult(providerapi.SecretGetResult{Value: strings.TrimSpace(string(output))})
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

func parseKeychainRef(ref string) (string, string, error) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("keychain ref must be service/account")
	}
	return parts[0], parts[1], nil
}
