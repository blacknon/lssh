package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/connectorruntime"
	"github.com/blacknon/lssh/internal/termenv"
)

func (r *Run) UsesConnector(server string) bool {
	return r.Conf.ServerUsesConnector(server)
}

func (r *Run) usesConnector(server string) bool {
	return r.UsesConnector(server)
}

func (r *Run) ProviderBridgeDialer(server string) connectorruntime.Dialer {
	return r.providerBridgeDialer(server)
}

func (r *Run) RunConnectorShell(server string) error {
	config := r.resolveShellConfig(server)
	return r.runConnectorShellWithConfig(server, config)
}

func (r *Run) runConnectorShellWithConfig(server string, config conf.ServerConfig) error {
	prepared, err := r.prepareConnectorOperation(server, conf.ConnectorOperation{Name: "shell"})
	if err != nil {
		return err
	}
	if prepared.ConnectorName == "" || prepared.ConnectorName == "ssh" {
		return fmt.Errorf("server %q does not use an external connector", server)
	}
	if r.TunnelEnabled {
		return fmt.Errorf("server %q uses connector %q; ssh tunnel devices are not supported", server, prepared.ConnectorName)
	}
	if connectorHasForwarding(config) {
		if r.connectorForwardingSharesShell(config, prepared) {
			return r.runConnectorShellWithSharedForwarding(server, config, prepared)
		}
		return r.runConnectorForwarding(server, config, prepared.ConnectorName)
	}

	return r.runConnectorShellOnly(server, config, prepared)
}

func (r *Run) runConnectorShellOnly(server string, config conf.ServerConfig, prepared conf.PreparedConnector) error {
	r.PrintSelectServer()
	r.printProxy(server)
	if !prepared.Supported {
		return fmt.Errorf("server %q connector %q: %v", server, prepared.ConnectorName, prepared.Reason)
	}
	if connectorLocalRCEnabled(r, config) && !connectorPlanSupportsLocalRC(prepared) {
		return fmt.Errorf("server %q uses connector %q; localrc is not supported", server, prepared.ConnectorName)
	}

	return runConnectorWithPrePost(config, func() error {
		startupCommand := connectorLocalRCCommand(r, config)
		startupMarker := ""
		if startupCommand != "" {
			startupMarker = InteractiveLocalRCStartupMarker()
		}

		switch {
		case prepared.Command != nil:
			return runCommandPlanShell(*prepared.Command)
		case prepared.ManagedSSH != nil:
			return r.runConnectorManagedSSHTransportShell(server, config)
		case prepared.ProviderManagedPlan != nil:
			return runNativeInteractiveSession(func() error {
				return r.providerManagedExecutor().RunShell(context.Background(), connectorruntime.ShellRequest{
					Server:         server,
					Plan:           *prepared.ProviderManagedPlan,
					LocalRCCommand: startupCommand,
					StartupMarker:  startupMarker,
					Dialer:         r.providerBridgeDialer(server),
					Stream: connectorruntime.StreamConfig{
						Stdin:  os.Stdin,
						Stdout: os.Stdout,
						Stderr: os.Stderr,
					},
				})
			})
		default:
			return fmt.Errorf("server %q connector %q returned unsupported plan kind %q", server, prepared.ConnectorName, prepared.PlanKind)
		}
	})
}

func (r *Run) runConnectorShellWithSharedForwarding(server string, config conf.ServerConfig, prepared conf.PreparedConnector) error {
	if !prepared.Supported {
		return fmt.Errorf("server %q connector %q: %v", server, prepared.ConnectorName, prepared.Reason)
	}
	if connectorLocalRCEnabled(r, config) && !connectorPlanSupportsLocalRC(prepared) {
		return fmt.Errorf("server %q uses connector %q; localrc is not supported", server, prepared.ConnectorName)
	}

	r.PrintSelectServer()
	r.printProxy(server)
	switch {
	case strings.TrimSpace(config.DynamicPortForward) != "":
		r.printDynamicPortForward(config.DynamicPortForward)
	case strings.TrimSpace(config.HTTPDynamicPortForward) != "":
		r.printHTTPDynamicPortForward(config.HTTPDynamicPortForward)
	}

	return runConnectorWithPrePost(config, func() error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		startupCommand := connectorLocalRCCommand(r, config)
		startupMarker := ""
		if startupCommand != "" {
			startupMarker = InteractiveLocalRCStartupMarker()
		}

		forwardErrCh := make(chan error, 1)
		go func() {
			switch {
			case strings.TrimSpace(config.DynamicPortForward) != "":
				forwardErrCh <- r.runConnectorDynamicPortForwardSession(ctx, server, config, prepared.ConnectorName, false)
			case strings.TrimSpace(config.HTTPDynamicPortForward) != "":
				forwardErrCh <- r.runConnectorHTTPDynamicPortForwardSession(ctx, server, config, prepared.ConnectorName, false)
			default:
				forwardErrCh <- nil
			}
		}()

		var shellErr error
		switch {
		case prepared.ManagedSSH != nil:
			shellErr = r.runConnectorManagedSSHTransportShell(server, config)
		case prepared.ProviderManagedPlan != nil:
			shellErr = runNativeInteractiveSession(func() error {
				return r.providerManagedExecutor().RunShell(context.Background(), connectorruntime.ShellRequest{
					Server:         server,
					Plan:           *prepared.ProviderManagedPlan,
					LocalRCCommand: startupCommand,
					StartupMarker:  startupMarker,
					Dialer:         r.providerBridgeDialer(server),
					Stream: connectorruntime.StreamConfig{
						Stdin:  os.Stdin,
						Stdout: os.Stdout,
						Stderr: os.Stderr,
					},
				})
			})
		default:
			cancel()
			<-forwardErrCh
			return fmt.Errorf("connector %q is not supported in the core runtime yet", prepared.ConnectorName)
		}
		cancel()
		forwardErr := <-forwardErrCh
		if shellErr != nil {
			return shellErr
		}
		if forwardErr != nil {
			return forwardErr
		}
		return nil
	})
}

func (r *Run) runConnectorShell(server string) error {
	return r.RunConnectorShell(server)
}

func (r *Run) RunConnectorLocalPortForward(server string) error {
	config := r.resolveShellConfig(server)
	return r.runConnectorLocalPortForward(server, config, r.Conf.ServerConnectorName(server))
}

func (r *Run) RunConnectorCommand(server string, command []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	operation := conf.ConnectorOperation{
		Name:    "exec",
		Command: append([]string(nil), command...),
	}
	return r.runConnectorCommandOperation(server, operation, stdin, stdout, stderr)
}

func (r *Run) RunConnectorCommandLine(server, commandLine string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	operation := conf.ConnectorOperation{
		Name: "exec",
		Options: map[string]interface{}{
			"command_line": commandLine,
		},
	}
	return r.runConnectorCommandOperation(server, operation, stdin, stdout, stderr)
}

func (r *Run) runConnectorCommandOperation(server string, operation conf.ConnectorOperation, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	prepared, err := r.prepareConnectorOperation(server, operation)
	if err != nil {
		return 0, err
	}
	if !prepared.Supported {
		return 0, fmt.Errorf("server %q connector %q: %v", server, prepared.ConnectorName, prepared.Reason)
	}

	switch {
	case prepared.Command != nil:
		return runCommandPlanExec(*prepared.Command, stdin, stdout, stderr)
	case prepared.ManagedSSH != nil:
		return r.runConnectorManagedSSHTransportExec(server, r.Conf.Server[server], operation.Command, connectorOptionString(operation.Options, "command_line"), stdin, stdout, stderr, operation.Name == "exec_pty")
	case prepared.ProviderManagedPlan != nil:
		return r.providerManagedExecutor().RunExec(context.Background(), connectorruntime.ExecRequest{
			Server: server,
			Plan:   *prepared.ProviderManagedPlan,
			Dialer: r.providerBridgeDialer(server),
			Stream: connectorruntime.StreamConfig{
				Stdin:  stdin,
				Stdout: stdout,
				Stderr: stderr,
			},
		})
	default:
		return 0, fmt.Errorf("connector %q does not support exec in the core runtime yet", prepared.ConnectorName)
	}
}

func (r *Run) providerManagedExecutor() connectorruntime.Executor {
	if r.ConnectorRuntime != nil {
		return r.ConnectorRuntime
	}
	return conf.NewProviderRuntimeExecutor(&r.Conf)
}

func (r *Run) runConnectorCommand(server string, stdout, stderr io.Writer) (int, error) {
	return r.RunConnectorCommand(server, r.ExecCmd, nil, stdout, stderr)
}

func (r *Run) connectorShellOperation() (conf.ConnectorOperation, error) {
	options := map[string]interface{}{}
	if r.ConnectorAttachSession != "" {
		options["attach"] = true
		options["session_id"] = r.ConnectorAttachSession
	}
	if r.ConnectorDetach {
		options["detach"] = true
	}

	operation := conf.ConnectorOperation{Name: "shell"}
	if len(options) > 0 {
		operation.Options = options
	}
	return operation, nil
}

func runCommandPlanShell(plan conf.ConnectorCommandPlan) error {
	if strings.TrimSpace(plan.Program) == "" {
		return fmt.Errorf("connector command plan is missing program")
	}

	cmd := exec.CommandContext(context.Background(), plan.Program, plan.Args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = mergedCommandPlanEnv(plan.Env)
	return cmd.Run()
}

func runCommandPlanExec(plan conf.ConnectorCommandPlan, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if strings.TrimSpace(plan.Program) == "" {
		return 0, fmt.Errorf("connector command plan is missing program")
	}

	cmd := exec.CommandContext(context.Background(), plan.Program, plan.Args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = mergedCommandPlanEnv(plan.Env)
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 0, err
}

func (r *Run) runConnectorForwarding(server string, config conf.ServerConfig, connectorName string) error {
	mode, err := connectorForwardMode(config)
	if err != nil {
		return fmt.Errorf("server %q uses connector %q; %w", server, connectorName, err)
	}

	switch mode {
	case "local":
		return r.runConnectorLocalPortForward(server, config, connectorName)
	case "dynamic":
		return r.runConnectorDynamicPortForward(server, config, connectorName)
	case "http-dynamic":
		return r.runConnectorHTTPDynamicPortForward(server, config, connectorName)
	default:
		return fmt.Errorf("server %q uses connector %q; unsupported forwarding mode %q", server, connectorName, mode)
	}
}

func (r *Run) runConnectorLocalPortForward(server string, config conf.ServerConfig, connectorName string) error {
	spec, err := connectorLocalForwardSpec(config)
	if err != nil {
		return fmt.Errorf("server %q uses connector %q; %w", server, connectorName, err)
	}
	if r.X11 || r.X11Trusted {
		return fmt.Errorf("server %q uses connector %q; X11 forwarding is not supported with local port forwarding", server, connectorName)
	}

	r.PrintSelectServer()
	r.printPortForward(spec.Mode, spec.Local, spec.Remote)

	operation := conf.ConnectorOperation{
		Name: "port_forward_local",
		Options: map[string]interface{}{
			"listen_host": spec.ListenHost,
			"listen_port": spec.ListenPort,
			"target_host": spec.TargetHost,
			"target_port": spec.TargetPort,
		},
	}

	prepared, err := r.prepareConnectorOperation(server, operation)
	if err != nil {
		return err
	}
	if !prepared.Supported {
		return fmt.Errorf("server %q connector %q: %v", server, prepared.ConnectorName, prepared.Reason)
	}

	return runConnectorWithPrePost(config, func() error {
		switch {
		case prepared.Command != nil:
			return runCommandPlanShell(*prepared.Command)
		case prepared.ManagedSSH != nil:
			return r.runConnectorManagedSSHLocalPortForward(server, config)
		default:
			return fmt.Errorf("server %q connector %q returned unsupported plan kind %q", server, prepared.ConnectorName, prepared.PlanKind)
		}
	})
}

func mergedCommandPlanEnv(env map[string]string) []string {
	return termenv.MergeEnv(env)
}

func copyConnectorStream(wg *sync.WaitGroup, dst io.Writer, src io.Reader) {
	defer wg.Done()
	_, _ = io.Copy(dst, src)
}

func connectorHasForwarding(config conf.ServerConfig) bool {
	return len(config.Forwards) > 0 ||
		config.DynamicPortForward != "" ||
		config.ReverseDynamicPortForward != "" ||
		config.HTTPDynamicPortForward != "" ||
		config.HTTPReverseDynamicPortForward != "" ||
		config.NFSDynamicForwardPort != "" ||
		config.NFSReverseDynamicForwardPort != "" ||
		config.SMBDynamicForwardPort != "" ||
		config.SMBReverseDynamicForwardPort != ""
}

func (r *Run) connectorForwardingSharesShell(config conf.ServerConfig, prepared conf.PreparedConnector) bool {
	if r == nil || r.IsNone {
		return false
	}
	if !prepared.SharedShellForward {
		return false
	}
	if strings.TrimSpace(config.DynamicPortForward) != "" && strings.TrimSpace(config.HTTPDynamicPortForward) != "" {
		return false
	}
	return (strings.TrimSpace(config.DynamicPortForward) != "" || strings.TrimSpace(config.HTTPDynamicPortForward) != "") &&
		len(config.Forwards) == 0 &&
		config.ReverseDynamicPortForward == "" &&
		config.HTTPReverseDynamicPortForward == "" &&
		config.NFSDynamicForwardPort == "" &&
		config.NFSReverseDynamicForwardPort == "" &&
		config.SMBDynamicForwardPort == "" &&
		config.SMBReverseDynamicForwardPort == ""
}

func (r *Run) resolveShellConfig(server string) conf.ServerConfig {
	config := r.Conf.Server[server]
	config = r.setPortForwards(server, config)

	if r.DynamicPortForward != "" {
		config.DynamicPortForward = r.DynamicPortForward
	}
	if r.ReverseDynamicPortForward != "" {
		config.ReverseDynamicPortForward = r.ReverseDynamicPortForward
	}
	if r.HTTPDynamicPortForward != "" {
		config.HTTPDynamicPortForward = r.HTTPDynamicPortForward
	}
	if r.HTTPReverseDynamicPortForward != "" {
		config.HTTPReverseDynamicPortForward = r.HTTPReverseDynamicPortForward
	}
	if r.NFSDynamicForwardPort != "" {
		config.NFSDynamicForwardPort = r.NFSDynamicForwardPort
	}
	if r.NFSDynamicForwardPath != "" {
		config.NFSDynamicForwardPath = r.NFSDynamicForwardPath
	}
	if r.SMBDynamicForwardPort != "" {
		config.SMBDynamicForwardPort = r.SMBDynamicForwardPort
	}
	if r.SMBDynamicForwardPath != "" {
		config.SMBDynamicForwardPath = r.SMBDynamicForwardPath
	}
	if r.NFSReverseDynamicForwardPort != "" {
		config.NFSReverseDynamicForwardPort = r.NFSReverseDynamicForwardPort
	}
	if r.NFSReverseDynamicForwardPath != "" {
		config.NFSReverseDynamicForwardPath = r.NFSReverseDynamicForwardPath
	}
	if r.SMBReverseDynamicForwardPort != "" {
		config.SMBReverseDynamicForwardPort = r.SMBReverseDynamicForwardPort
	}
	if r.SMBReverseDynamicForwardPath != "" {
		config.SMBReverseDynamicForwardPath = r.SMBReverseDynamicForwardPath
	}
	if r.IsBashrc {
		config.LocalRcUse = "yes"
	}
	if r.IsNotBashrc {
		config.LocalRcUse = "no"
	}

	return config
}

type connectorPortForwardSpec struct {
	Mode       string
	Local      string
	Remote     string
	ListenHost string
	ListenPort string
	TargetHost string
	TargetPort string
}

func connectorForwardMode(config conf.ServerConfig) (string, error) {
	localForwardConfigured := len(config.Forwards) > 0
	dynamicForwardConfigured := strings.TrimSpace(config.DynamicPortForward) != ""
	httpDynamicForwardConfigured := strings.TrimSpace(config.HTTPDynamicPortForward) != ""

	if localForwardConfigured && (dynamicForwardConfigured || httpDynamicForwardConfigured) {
		return "", fmt.Errorf("combining local (-L) and dynamic (-D/-d) port forwarding is not supported")
	}
	if dynamicForwardConfigured && httpDynamicForwardConfigured {
		return "", fmt.Errorf("combining dynamic (-D) and http dynamic (-d) port forwarding is not supported")
	}
	if config.ReverseDynamicPortForward != "" {
		return "", fmt.Errorf("reverse dynamic port forwarding (-R port) is not supported yet")
	}
	if config.HTTPDynamicPortForward != "" {
		return "http-dynamic", nil
	}
	if config.HTTPReverseDynamicPortForward != "" {
		return "", fmt.Errorf("http reverse dynamic port forwarding (-r) is not supported yet")
	}
	if config.NFSDynamicForwardPort != "" || config.NFSDynamicForwardPath != "" {
		return "", fmt.Errorf("nfs dynamic forwarding (-M) is not supported yet")
	}
	if config.NFSReverseDynamicForwardPort != "" || config.NFSReverseDynamicForwardPath != "" {
		return "", fmt.Errorf("nfs reverse dynamic forwarding (-m) is not supported yet")
	}
	if config.SMBDynamicForwardPort != "" || config.SMBDynamicForwardPath != "" {
		return "", fmt.Errorf("smb dynamic forwarding (-S) is not supported yet")
	}
	if config.SMBReverseDynamicForwardPort != "" || config.SMBReverseDynamicForwardPath != "" {
		return "", fmt.Errorf("smb reverse dynamic forwarding (-s) is not supported yet")
	}

	if localForwardConfigured {
		return "local", nil
	}
	if dynamicForwardConfigured {
		return "dynamic", nil
	}
	return "", fmt.Errorf("forwarding is not configured")
}

func connectorLocalForwardSpec(config conf.ServerConfig) (connectorPortForwardSpec, error) {
	if config.DynamicPortForward != "" ||
		config.ReverseDynamicPortForward != "" ||
		config.HTTPDynamicPortForward != "" ||
		config.HTTPReverseDynamicPortForward != "" ||
		config.NFSDynamicForwardPort != "" ||
		config.NFSReverseDynamicForwardPort != "" ||
		config.SMBDynamicForwardPort != "" ||
		config.SMBReverseDynamicForwardPort != "" {
		return connectorPortForwardSpec{}, fmt.Errorf("only simple local tcp port forwarding is supported")
	}
	if len(config.Forwards) != 1 {
		return connectorPortForwardSpec{}, fmt.Errorf("exactly one local port forward is required")
	}

	forward := config.Forwards[0]
	if forward == nil {
		return connectorPortForwardSpec{}, fmt.Errorf("local port forward is not configured")
	}
	if strings.ToUpper(forward.Mode) != "L" {
		return connectorPortForwardSpec{}, fmt.Errorf("only local port forwarding is supported")
	}
	if forward.LocalNetwork != "" && forward.LocalNetwork != "tcp" {
		return connectorPortForwardSpec{}, fmt.Errorf("local port forward network %q is not supported", forward.LocalNetwork)
	}
	if forward.RemoteNetwork != "" && forward.RemoteNetwork != "tcp" {
		return connectorPortForwardSpec{}, fmt.Errorf("remote port forward network %q is not supported", forward.RemoteNetwork)
	}

	listenHost, listenPort, err := net.SplitHostPort(forward.Local)
	if err != nil {
		return connectorPortForwardSpec{}, fmt.Errorf("invalid local port forward endpoint %q: %w", forward.Local, err)
	}
	targetHost, targetPort, err := net.SplitHostPort(forward.Remote)
	if err != nil {
		return connectorPortForwardSpec{}, fmt.Errorf("invalid remote port forward endpoint %q: %w", forward.Remote, err)
	}

	return connectorPortForwardSpec{
		Mode:       "L",
		Local:      forward.Local,
		Remote:     forward.Remote,
		ListenHost: listenHost,
		ListenPort: listenPort,
		TargetHost: targetHost,
		TargetPort: targetPort,
	}, nil
}

func connectorLocalRCEnabled(r *Run, config conf.ServerConfig) bool {
	return LocalRCEnabled(r, config)
}

func connectorLocalRCCommand(r *Run, config conf.ServerConfig) string {
	return LocalRCCommand(r, config)
}

func connectorPlanSupportsLocalRC(prepared conf.PreparedConnector) bool {
	return prepared.SupportsLocalRC || prepared.ManagedSSH != nil
}

func connectorErrorString(server string, err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return fmt.Sprintf("%s :: connector execution failed", server)
	}
	return fmt.Sprintf("%s :: %s", server, message)
}

func connectorOutputWriters(r *Run, server string, single bool) (io.Writer, io.Writer) {
	if single && r.IsTerm {
		return os.Stdout, os.Stderr
	}
	o := r.createCommandOutput(server)
	return o.NewWriter(), o.NewWriter()
}

func runConnectorWithPrePost(config conf.ServerConfig, run func() error) error {
	if config.PreCmd != "" {
		execLocalCommand(config.PreCmd)
	}
	if config.PostCmd != "" {
		defer execLocalCommand(config.PostCmd)
	}
	return run()
}

func connectorOptionString(raw map[string]interface{}, key string) string {
	if raw == nil {
		return ""
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func (r *Run) prepareConnectorOperation(server string, operation conf.ConnectorOperation) (conf.PreparedConnector, error) {
	connectorName := r.Conf.ServerConnectorName(server)
	if connectorName == "" || connectorName == "ssh" {
		return conf.PreparedConnector{}, fmt.Errorf("server %q does not use an external connector", server)
	}

	return r.Conf.PrepareConnectorRuntime(server, operation)
}
