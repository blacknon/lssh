package conf

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/blacknon/lssh/internal/connectorruntime"
	"github.com/blacknon/lssh/providerapi"
)

type providerRuntimeExecutor struct {
	config *Config
}

func NewProviderRuntimeExecutor(config *Config) connectorruntime.Executor {
	return providerRuntimeExecutor{config: config}
}

func (e providerRuntimeExecutor) RunShell(ctx context.Context, request connectorruntime.ShellRequest) error {
	params, err := e.runtimeParams(request.Server, request.Plan)
	if err != nil {
		return err
	}
	params.LocalRCCommand = request.LocalRCCommand
	params.StartupMarker = request.StartupMarker

	return e.config.callProviderRuntime(
		ctx,
		params.Provider,
		providerapi.MethodConnectorShell,
		params,
		request.Stream.Stdin,
		request.Stream.Stdout,
		request.Stream.Stderr,
		nil,
	)
}

func (e providerRuntimeExecutor) RunExec(ctx context.Context, request connectorruntime.ExecRequest) (int, error) {
	params, err := e.runtimeParams(request.Server, request.Plan)
	if err != nil {
		return 0, err
	}

	var result providerapi.ConnectorExecResult
	err = e.config.callProviderRuntime(
		ctx,
		params.Provider,
		providerapi.MethodConnectorExec,
		params,
		request.Stream.Stdin,
		request.Stream.Stdout,
		request.Stream.Stderr,
		&result,
	)
	return result.ExitCode, err
}

func (e providerRuntimeExecutor) runtimeParams(server string, plan providerapi.ConnectorPlan) (providerapi.ConnectorRuntimeParams, error) {
	serverConfig, ok := e.config.Server[server]
	if !ok {
		return providerapi.ConnectorRuntimeParams{}, fmt.Errorf("server %q is not configured", server)
	}

	connectorName := strings.TrimSpace(e.config.serverConnectorName(serverConfig))
	if connectorName == "" || connectorName == "ssh" {
		return providerapi.ConnectorRuntimeParams{}, fmt.Errorf("server %q does not use an external connector", server)
	}

	providerName, raw, err := e.config.resolveConnectorProvider(serverConfig, connectorName)
	if err != nil {
		return providerapi.ConnectorRuntimeParams{}, err
	}

	return providerapi.ConnectorRuntimeParams{
		Provider: providerName,
		Config:   raw,
		Target: providerapi.ConnectorTarget{
			Name:   server,
			Config: serverConfigToTOMLMap(serverConfig),
			Meta:   cloneProviderMeta(serverConfig.ProviderMeta),
		},
		Plan: plan,
	}, nil
}

func (c *Config) callProviderRuntime(ctx context.Context, name, method string, params interface{}, stdin io.Reader, stdout, stderr io.Writer, out interface{}) error {
	cmd, resultPath, err := c.prepareProviderRuntimeCommand(ctx, name, method, params, out != nil)
	if err != nil {
		return err
	}
	defer func() {
		if resultPath != "" {
			_ = os.Remove(resultPath)
		}
	}()

	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("provider %q: %w", name, err)
	}

	if out == nil {
		return nil
	}

	data, err := os.ReadFile(resultPath)
	if err != nil {
		return fmt.Errorf("provider %q runtime result: %w", name, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("provider %q decode runtime result: %w", name, err)
	}
	return nil
}

func (c *Config) PrepareProviderRuntimeCommand(ctx context.Context, server string, plan providerapi.ConnectorPlan, method, localRCCommand, startupMarker string, wantResult bool) (*exec.Cmd, string, error) {
	params, err := (providerRuntimeExecutor{config: c}).runtimeParams(server, plan)
	if err != nil {
		return nil, "", err
	}
	params.LocalRCCommand = localRCCommand
	params.StartupMarker = startupMarker
	return c.prepareProviderRuntimeCommand(ctx, params.Provider, method, params, wantResult)
}

func (c *Config) prepareProviderRuntimeCommand(ctx context.Context, name, method string, params interface{}, wantResult bool) (*exec.Cmd, string, error) {
	raw, ok := c.Provider[name]
	if !ok {
		return nil, "", fmt.Errorf("provider %q is not configured", name)
	}

	pluginName := providerString(raw, "plugin")
	if pluginName == "" {
		pluginName = name
	}

	path, err := resolveProviderExecutable(c.Providers, pluginName)
	if err != nil {
		return nil, "", fmt.Errorf("provider %q: %w", name, err)
	}

	if ctx == nil {
		ctx = context.Background()
	}

	req := providerapi.Request{
		Version: providerapi.Version,
		Method:  method,
		Params:  params,
	}
	input, err := json.Marshal(req)
	if err != nil {
		return nil, "", err
	}

	cmd := exec.CommandContext(ctx, path)
	cmd.Env = append(os.Environ(), providerapi.RequestEnvVar+"="+base64.StdEncoding.EncodeToString(input))

	resultPath := ""
	if wantResult {
		file, err := os.CreateTemp("", "lssh-provider-runtime-*.json")
		if err != nil {
			return nil, "", err
		}
		resultPath = file.Name()
		_ = file.Close()
		cmd.Env = append(cmd.Env, providerapi.ResultEnvVar+"="+resultPath)
	}
	return cmd, resultPath, nil
}
