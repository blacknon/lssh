package conf

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

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

	extraEnv, bridgeCloser, err := providerRuntimeBridgeEnv(ctx, request.Plan, request.Dialer)
	if err != nil {
		return err
	}
	if bridgeCloser != nil {
		defer bridgeCloser.Close()
	}

	return e.config.callProviderRuntime(
		ctx,
		params.Provider,
		providerapi.MethodConnectorShell,
		params,
		extraEnv,
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

	extraEnv, bridgeCloser, err := providerRuntimeBridgeEnv(ctx, request.Plan, request.Dialer)
	if err != nil {
		return 0, err
	}
	if bridgeCloser != nil {
		defer bridgeCloser.Close()
	}

	var result providerapi.ConnectorExecResult
	err = e.config.callProviderRuntime(
		ctx,
		params.Provider,
		providerapi.MethodConnectorExec,
		params,
		extraEnv,
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

func (c *Config) callProviderRuntime(ctx context.Context, name, method string, params interface{}, extraEnv []string, stdin io.Reader, stdout, stderr io.Writer, out interface{}) error {
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
	if len(extraEnv) > 0 {
		cmd.Env = append(cmd.Env, extraEnv...)
	}
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
	cmd, resultPath, bridgeCloser, err := c.PrepareProviderRuntimeCommandWithDialer(ctx, server, plan, method, localRCCommand, startupMarker, wantResult, nil)
	if err != nil {
		return nil, "", err
	}
	if bridgeCloser != nil {
		return nil, "", fmt.Errorf("unexpected bridge closer without dialer")
	}
	return cmd, resultPath, nil
}

func (c *Config) PrepareProviderRuntimeCommandWithDialer(ctx context.Context, server string, plan providerapi.ConnectorPlan, method, localRCCommand, startupMarker string, wantResult bool, dialer connectorruntime.Dialer) (*exec.Cmd, string, io.Closer, error) {
	params, err := (providerRuntimeExecutor{config: c}).runtimeParams(server, plan)
	if err != nil {
		return nil, "", nil, err
	}
	params.LocalRCCommand = localRCCommand
	params.StartupMarker = startupMarker

	cmd, resultPath, err := c.prepareProviderRuntimeCommand(ctx, params.Provider, method, params, wantResult)
	if err != nil {
		return nil, "", nil, err
	}

	extraEnv, bridgeCloser, err := providerRuntimeBridgeEnv(ctx, plan, dialer)
	if err != nil {
		return nil, "", nil, err
	}
	if len(extraEnv) > 0 {
		cmd.Env = append(cmd.Env, extraEnv...)
	}

	return cmd, resultPath, bridgeCloser, nil
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

func providerRuntimeBridgeEnv(ctx context.Context, plan providerapi.ConnectorPlan, dialer connectorruntime.Dialer) ([]string, io.Closer, error) {
	if dialer == nil {
		return nil, nil, nil
	}
	network := strings.TrimSpace(detailString(plan.Details, "runtime_dial_network"))
	address := strings.TrimSpace(detailString(plan.Details, "runtime_dial_address"))
	if network == "" || address == "" {
		return nil, nil, nil
	}

	bridge, err := startProviderRuntimeBridge(ctx, dialer, network, address)
	if err != nil {
		return nil, nil, err
	}
	return []string{providerapi.RuntimeBridgeAddrEnvVar + "=" + bridge.Address()}, bridge, nil
}

type providerRuntimeBridge struct {
	listener  net.Listener
	upstream  net.Conn
	closeOnce sync.Once
	done      chan struct{}
}

func startProviderRuntimeBridge(ctx context.Context, dialer connectorruntime.Dialer, network, address string) (*providerRuntimeBridge, error) {
	upstream, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = upstream.Close()
		return nil, err
	}

	bridge := &providerRuntimeBridge{
		listener: listener,
		upstream: upstream,
		done:     make(chan struct{}),
	}
	go bridge.serve()
	return bridge, nil
}

func (b *providerRuntimeBridge) Address() string {
	if b == nil || b.listener == nil {
		return ""
	}
	return b.listener.Addr().String()
}

func (b *providerRuntimeBridge) serve() {
	defer close(b.done)

	client, err := b.listener.Accept()
	if err != nil {
		_ = b.Close()
		return
	}
	defer client.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(client, b.upstream)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(b.upstream, client)
	}()
	wg.Wait()
	_ = b.Close()
}

func (b *providerRuntimeBridge) Close() error {
	if b == nil {
		return nil
	}
	var closeErr error
	b.closeOnce.Do(func() {
		if b.listener != nil {
			if err := b.listener.Close(); err != nil {
				closeErr = err
			}
		}
		if b.upstream != nil {
			if err := b.upstream.Close(); closeErr == nil {
				closeErr = err
			}
		}
	})
	return closeErr
}
