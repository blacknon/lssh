package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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

		command := providerbuiltin.StringSlice(params.Config, "command")
		if len(command) == 0 {
			path := providerbuiltin.String(params.Config, "path")
			if path != "" {
				command = []string{path}
			}
		}
		if len(command) == 0 {
			_ = providerbuiltin.WriteError("custom-script provider requires command or path")
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
			_ = providerbuiltin.WriteError(err.Error())
			os.Exit(1)
		}

		_ = providerbuiltin.WriteResult(providerapi.SecretGetResult{Value: strings.TrimRight(string(output), "\n")})
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
