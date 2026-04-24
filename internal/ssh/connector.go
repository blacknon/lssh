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
	"github.com/blacknon/lssh/internal/providerapi"
	"github.com/blacknon/lssh/internal/termenv"
	"github.com/blacknon/lssh/provider/connector/provider-connector-telnet/telnetlib"
	"github.com/blacknon/lssh/provider/connector/provider-connector-winrm/winrmlib"
	azurebastionconnector "github.com/blacknon/lssh/provider/inventory/provider-inventory-azure-compute/bastionconnector"
	gcpiapconnector "github.com/blacknon/lssh/provider/inventory/provider-inventory-gcp-compute/iapconnector"
	awseiceconnector "github.com/blacknon/lssh/provider/mixed/provider-mixed-aws-ec2/eiceconnector"
	ssmconnector "github.com/blacknon/lssh/provider/mixed/provider-mixed-aws-ec2/ssmconnector"
	"github.com/blacknon/lssh/provider/mixed/provider-mixed-aws-ec2/ssmsession"
	"golang.org/x/crypto/ssh/terminal"
)

func (r *Run) UsesConnector(server string) bool {
	return r.Conf.ServerUsesConnector(server)
}

func (r *Run) usesConnector(server string) bool {
	return r.UsesConnector(server)
}

func (r *Run) RunConnectorShell(server string) error {
	config := r.resolveShellConfig(server)
	return r.runConnectorShellWithConfig(server, config)
}

func (r *Run) runConnectorShellWithConfig(server string, config conf.ServerConfig) error {
	connectorName := r.Conf.ServerConnectorName(server)
	if connectorName == "" || connectorName == "ssh" {
		return fmt.Errorf("server %q does not use an external connector", server)
	}
	if r.TunnelEnabled {
		return fmt.Errorf("server %q uses connector %q; ssh tunnel devices are not supported", server, connectorName)
	}
	if connectorHasForwarding(config) {
		if r.connectorForwardingSharesShell(config, connectorName) {
			return r.runConnectorShellWithSharedForwarding(server, config, connectorName)
		}
		return r.runConnectorForwarding(server, config, connectorName)
	}

	return r.runConnectorShellOnly(server, config, connectorName)
}

func (r *Run) runConnectorShellOnly(server string, config conf.ServerConfig, connectorName string) error {
	r.PrintSelectServer()

	operation, err := r.connectorShellOperation(connectorName)
	if err != nil {
		return err
	}

	prepared, err := r.Conf.PrepareConnector(server, operation)
	if err != nil {
		return err
	}
	planConnector := connectorNameFromPlan(prepared.Plan, connectorName)
	if !prepared.Supported {
		return fmt.Errorf("server %q connector %q: %v", server, planConnector, prepared.Plan.Details["reason"])
	}
	if connectorLocalRCEnabled(r, config) && !connectorPlanSupportsLocalRC(prepared.Plan, planConnector) {
		return fmt.Errorf("server %q uses connector %q; localrc is not supported", server, planConnector)
	}

	return runConnectorWithPrePost(config, func() error {
		switch prepared.Plan.Kind {
		case "command":
			return runCommandPlanShell(prepared.Plan)
		case "provider-managed":
			return runProviderManagedShell(r, config, prepared.Plan, planConnector)
		default:
			return fmt.Errorf("server %q connector %q returned unsupported plan kind %q", server, planConnector, prepared.Plan.Kind)
		}
	})
}

func (r *Run) runConnectorShellWithSharedForwarding(server string, config conf.ServerConfig, connectorName string) error {
	operation, err := r.connectorShellOperation(connectorName)
	if err != nil {
		return err
	}

	prepared, err := r.Conf.PrepareConnector(server, operation)
	if err != nil {
		return err
	}
	planConnector := connectorNameFromPlan(prepared.Plan, connectorName)
	if !prepared.Supported {
		return fmt.Errorf("server %q connector %q: %v", server, planConnector, prepared.Plan.Details["reason"])
	}
	if connectorLocalRCEnabled(r, config) && !connectorPlanSupportsLocalRC(prepared.Plan, planConnector) {
		return fmt.Errorf("server %q uses connector %q; localrc is not supported", server, planConnector)
	}

	r.PrintSelectServer()
	switch {
	case strings.TrimSpace(config.DynamicPortForward) != "":
		r.printDynamicPortForward(config.DynamicPortForward)
	case strings.TrimSpace(config.HTTPDynamicPortForward) != "":
		r.printHTTPDynamicPortForward(config.HTTPDynamicPortForward)
	}

	return runConnectorWithPrePost(config, func() error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		forwardErrCh := make(chan error, 1)
		go func() {
			switch {
			case strings.TrimSpace(config.DynamicPortForward) != "":
				forwardErrCh <- r.runConnectorDynamicPortForwardSession(ctx, server, config, connectorName, false)
			case strings.TrimSpace(config.HTTPDynamicPortForward) != "":
				forwardErrCh <- r.runConnectorHTTPDynamicPortForwardSession(ctx, server, config, connectorName, false)
			default:
				forwardErrCh <- nil
			}
		}()

		shellErr := runProviderManagedShell(r, config, prepared.Plan, planConnector)
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
	operation := providerapi.ConnectorOperation{
		Name:    "exec",
		Command: append([]string(nil), command...),
	}
	return r.runConnectorCommandOperation(server, operation, stdin, stdout, stderr)
}

func (r *Run) RunConnectorCommandLine(server, commandLine string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	operation := providerapi.ConnectorOperation{
		Name: "exec",
		Options: map[string]interface{}{
			"command_line": commandLine,
		},
	}
	return r.runConnectorCommandOperation(server, operation, stdin, stdout, stderr)
}

func (r *Run) runConnectorCommandOperation(server string, operation providerapi.ConnectorOperation, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	connectorName := r.Conf.ServerConnectorName(server)
	if connectorName == "" || connectorName == "ssh" {
		return 0, fmt.Errorf("server %q does not use an external connector", server)
	}

	prepared, err := r.Conf.PrepareConnector(server, operation)
	if err != nil {
		return 0, err
	}
	planConnector := connectorNameFromPlan(prepared.Plan, connectorName)
	if !prepared.Supported {
		return 0, fmt.Errorf("server %q connector %q: %v", server, planConnector, prepared.Plan.Details["reason"])
	}

	switch prepared.Plan.Kind {
	case "command":
		return runCommandPlanExec(prepared.Plan, stdin, stdout, stderr)
	case "provider-managed":
		if connectorManagedSSHRuntime(prepared.Plan, planConnector) {
			return r.runConnectorManagedSSHTransportExec(server, r.Conf.Server[server], operation.Command, connectorOptionString(operation.Options, "command_line"), stdin, stdout, stderr, operation.Name == "exec_pty")
		}
		return runProviderManagedExec(prepared.Plan, planConnector, stdin, stdout, stderr)
	default:
		return 0, fmt.Errorf("server %q connector %q returned unsupported plan kind %q", server, planConnector, prepared.Plan.Kind)
	}
}

func (r *Run) runConnectorCommand(server string, stdout, stderr io.Writer) (int, error) {
	return r.RunConnectorCommand(server, r.ExecCmd, nil, stdout, stderr)
}

func (r *Run) connectorShellOperation(connectorName string) (providerapi.ConnectorOperation, error) {
	options := map[string]interface{}{}
	if r.ConnectorAttachSession != "" {
		options["attach"] = true
		options["session_id"] = r.ConnectorAttachSession
	}
	if r.ConnectorDetach {
		options["detach"] = true
	}

	if len(options) > 0 && connectorName != "aws-ssm" {
		return providerapi.ConnectorOperation{}, fmt.Errorf("connector %q does not support --attach/--detach", connectorName)
	}

	operation := providerapi.ConnectorOperation{Name: "shell"}
	if len(options) > 0 {
		operation.Options = options
	}
	return operation, nil
}

func connectorNameFromPlan(plan providerapi.ConnectorPlan, fallback string) string {
	if value := detailString(plan.Details, "connector"); value != "" {
		return value
	}
	return fallback
}

func runProviderManagedShell(r *Run, config conf.ServerConfig, plan providerapi.ConnectorPlan, connectorName string) error {
	if connectorManagedSSHRuntime(plan, connectorName) {
		server := ""
		if len(r.ServerList) > 0 {
			server = r.ServerList[0]
		}
		return r.runConnectorManagedSSHTransportShell(server, config)
	}

	switch connectorName {
	case "aws-ssm":
		shellConfig, err := ssmconnector.ShellConfigFromPlan(plan)
		if err != nil {
			return err
		}
		if shellConfig.Runtime == "native" {
			if shellConfig.SessionAction != "start" {
				return fmt.Errorf("aws ssm native shell does not support session_action %q yet", shellConfig.SessionAction)
			}
			startupCommand := connectorLocalRCCommand(r, config)
			startupMarker := ""
			if startupCommand != "" {
				startupMarker = InteractiveLocalRCStartupMarker()
			}
			return runNativeSSMShell(ssmsession.Config{
				InstanceID:             shellConfig.InstanceID,
				Region:                 shellConfig.Region,
				Platform:               shellConfig.Platform,
				Profile:                shellConfig.Profile,
				SharedConfigFiles:      append([]string(nil), shellConfig.SharedConfigFiles...),
				SharedCredentialsFiles: append([]string(nil), shellConfig.SharedCredentialsFiles...),
				DocumentName:           shellConfig.DocumentName,
				StartupCommand:         startupCommand,
				StartupMarker:          startupMarker,
			}, os.Stdin, os.Stdout, os.Stderr)
		}
		if shellConfig.SessionAction == "detach" {
			sessionID, err := ssmconnector.StartDetachedShell(context.Background(), shellConfig)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Detached Session:%s\n", sessionID)
			return nil
		}
		return ssmconnector.StartShell(context.Background(), shellConfig)
	case "winrm":
		return runWinRMShell(plan)
	case "telnet":
		return runTelnetShell(plan)
	default:
		return fmt.Errorf("connector %q is not supported yet", connectorName)
	}
}

func runProviderManagedExec(plan providerapi.ConnectorPlan, connectorName string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if connectorManagedSSHRuntime(plan, connectorName) {
		return 0, fmt.Errorf("connector %q managed ssh exec requires run context", connectorName)
	}

	switch connectorName {
	case "aws-ssm":
		if detailString(plan.Details, "shell_runtime") == "native" && detailString(plan.Details, "command_line") != "" {
			cfg, err := ssmconnector.CommandConfigFromPlan(plan)
			if err != nil {
				return 0, err
			}
			return ssmsession.RunStreamCommand(context.Background(), ssmsession.Config{
				InstanceID:             cfg.InstanceID,
				Region:                 cfg.Region,
				Platform:               cfg.Platform,
				Profile:                cfg.Profile,
				SharedConfigFiles:      append([]string(nil), cfg.SharedConfigFiles...),
				SharedCredentialsFiles: append([]string(nil), cfg.SharedCredentialsFiles...),
			}, detailString(plan.Details, "command_line"), stdin, stdout, stderr)
		}
		cmdConfig, err := ssmconnector.CommandConfigFromPlan(plan)
		if err != nil {
			return 0, err
		}
		return ssmconnector.RunCommand(context.Background(), cmdConfig, stdout, stderr)
	case "winrm":
		return runWinRMExec(plan, stdout, stderr)
	default:
		return 0, fmt.Errorf("connector %q does not support exec in the core runtime yet", connectorName)
	}
}

func runProviderManagedLocalPortForward(plan providerapi.ConnectorPlan, connectorName string) error {
	switch connectorName {
	case "aws-ssm":
		cfg, err := ssmconnector.PortForwardLocalConfigFromPlan(plan)
		if err != nil {
			return err
		}
		if cfg.Runtime == "native" {
			return runNativeSSMLocalPortForward(plan, cfg)
		}
		return ssmconnector.StartLocalPortForward(context.Background(), cfg)
	default:
		return fmt.Errorf("connector %q does not support local port forwarding in the core runtime yet", connectorName)
	}
}

func dialProviderManagedTransport(plan providerapi.ConnectorPlan, connectorName string) (net.Conn, error) {
	switch connectorName {
	case "aws-ssm":
		cfg, err := ssmconnector.PortForwardDialConfigFromPlan(plan)
		if err != nil {
			return nil, err
		}
		if cfg.Runtime == "native" && !awsSSMDynamicDialNeedsPluginFallback(plan) {
			return ssmsession.DialTarget(context.Background(), ssmsession.Config{
				InstanceID:             cfg.InstanceID,
				Region:                 cfg.Region,
				Platform:               cfg.Platform,
				Profile:                cfg.Profile,
				SharedConfigFiles:      append([]string(nil), cfg.SharedConfigFiles...),
				SharedCredentialsFiles: append([]string(nil), cfg.SharedCredentialsFiles...),
				DocumentName:           cfg.DocumentName,
				PortForwardHost:        cfg.TargetHost,
				PortForwardPort:        cfg.TargetPort,
				PortForwardLocalPort:   "0",
			})
		}
		if cfg.Runtime == "native" {
			cfg.Runtime = "plugin"
		}
		return ssmconnector.DialTarget(context.Background(), cfg)
	case "aws-eice":
		cfg, err := awseiceconnector.ConfigFromPlan(plan)
		if err != nil {
			return nil, err
		}
		return awseiceconnector.DialTarget(context.Background(), cfg)
	case "gcp-iap":
		cfg, err := gcpiapconnector.ConfigFromPlan(plan)
		if err != nil {
			return nil, err
		}
		return gcpiapconnector.DialTarget(context.Background(), cfg)
	case "azure-bastion":
		cfg, err := azurebastionconnector.ConfigFromPlan(plan)
		if err != nil {
			return nil, err
		}
		return azurebastionconnector.DialTarget(context.Background(), cfg)
	default:
		return nil, fmt.Errorf("connector %q does not support dial transport in the core runtime yet", connectorName)
	}
}

func awsSSMDynamicDialNeedsPluginFallback(plan providerapi.ConnectorPlan) bool {
	return detailString(plan.Details, "connector") == "aws-ssm" &&
		detailString(plan.Details, "session_mode") == "tcp-dial-transport"
}

func runCommandPlanShell(plan providerapi.ConnectorPlan) error {
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

func runCommandPlanExec(plan providerapi.ConnectorPlan, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
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

	operation := providerapi.ConnectorOperation{
		Name: "port_forward_local",
		Options: map[string]interface{}{
			"listen_host": spec.ListenHost,
			"listen_port": spec.ListenPort,
			"target_host": spec.TargetHost,
			"target_port": spec.TargetPort,
		},
	}

	prepared, err := r.Conf.PrepareConnector(server, operation)
	if err != nil {
		return err
	}
	planConnector := connectorNameFromPlan(prepared.Plan, connectorName)
	if !prepared.Supported {
		return fmt.Errorf("server %q connector %q: %v", server, planConnector, prepared.Plan.Details["reason"])
	}

	return runConnectorWithPrePost(config, func() error {
		switch prepared.Plan.Kind {
		case "command":
			return runCommandPlanShell(prepared.Plan)
		case "provider-managed":
			if connectorManagedSSHRuntime(prepared.Plan, planConnector) {
				return r.runConnectorManagedSSHLocalPortForward(server, config)
			}
			return runProviderManagedLocalPortForward(prepared.Plan, planConnector)
		default:
			return fmt.Errorf("server %q connector %q returned unsupported plan kind %q", server, planConnector, prepared.Plan.Kind)
		}
	})
}

func mergedCommandPlanEnv(env map[string]string) []string {
	return termenv.MergeEnv(env)
}

func runWinRMShell(plan providerapi.ConnectorPlan) error {
	cfg, err := winrmlib.ConfigFromPlanDetails(plan.Details)
	if err != nil {
		return err
	}

	connect, err := winrmlib.CreateConnect(cfg)
	if err != nil {
		return err
	}
	terminal, err := connect.OpenShell()
	if err != nil {
		return err
	}
	defer terminal.Close()

	return streamInteractiveSession(terminal.Stdin(), terminal.Stdout(), terminal.Stderr(), terminal.Wait)
}

func runTelnetShell(plan providerapi.ConnectorPlan) error {
	cfg, err := telnetlib.ConfigFromPlanDetails(plan.Details)
	if err != nil {
		return err
	}

	terminal, err := telnetlib.OpenShell(cfg, "xterm-256color", 80, 24)
	if err != nil {
		return err
	}
	defer terminal.Close()

	return streamInteractiveSession(terminal.Stdin(), terminal.Stdout(), terminal.Stderr(), terminal.Wait)
}

func runWinRMExec(plan providerapi.ConnectorPlan, stdout, stderr io.Writer) (int, error) {
	cfg, err := winrmlib.ConfigFromPlanDetails(plan.Details)
	if err != nil {
		return 0, err
	}

	connect, err := winrmlib.CreateConnect(cfg)
	if err != nil {
		return 0, err
	}
	commandLine := detailString(plan.Details, "command_line")
	if commandLine == "" {
		commandLine = strings.Join(detailStringSlice(plan.Details, "command"), " ")
	}
	command, err := connect.ExecuteCommand(commandLine)
	if err != nil {
		return 0, err
	}
	defer command.Close()

	var wg sync.WaitGroup
	if stdout != nil {
		wg.Add(1)
		go copyConnectorStream(&wg, stdout, command.Stdout())
	}
	if stderr != nil {
		wg.Add(1)
		go copyConnectorStream(&wg, stderr, command.Stderr())
	}

	waitErr := command.Wait()
	wg.Wait()
	return command.ExitCode(), waitErr
}

func streamInteractiveSession(stdin io.Writer, stdout, stderr io.Reader, wait func() error) error {
	var wg sync.WaitGroup
	if stdout != nil {
		wg.Add(1)
		go copyConnectorStream(&wg, os.Stdout, stdout)
	}
	if stderr != nil {
		wg.Add(1)
		go copyConnectorStream(&wg, os.Stderr, stderr)
	}
	if stdin != nil {
		go func() {
			_, _ = io.Copy(stdin, os.Stdin)
		}()
	}

	err := wait()
	wg.Wait()
	return err
}

func copyConnectorStream(wg *sync.WaitGroup, dst io.Writer, src io.Reader) {
	defer wg.Done()
	_, _ = io.Copy(dst, src)
}

func runNativeInteractiveSession(run func() error) error {
	fd := int(os.Stdin.Fd())
	if !terminal.IsTerminal(fd) {
		return run()
	}

	state, err := terminal.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer func() {
		_ = terminal.Restore(fd, state)
	}()

	return run()
}

func detailString(details map[string]interface{}, key string) string {
	if details == nil {
		return ""
	}
	if value, ok := details[key]; ok && value != nil {
		return fmt.Sprint(value)
	}
	return ""
}

func detailStringSlice(details map[string]interface{}, key string) []string {
	if details == nil {
		return nil
	}
	raw, ok := details[key]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if item == nil {
				continue
			}
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return []string{fmt.Sprint(typed)}
	}
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

func (r *Run) connectorForwardingSharesShell(config conf.ServerConfig, connectorName string) bool {
	if r == nil || r.IsNone {
		return false
	}
	if connectorName != "aws-ssm" {
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
	if r == nil {
		return config.LocalRcUse == "yes"
	}
	if r.IsNotBashrc {
		return false
	}
	if r.IsBashrc {
		return true
	}
	return config.LocalRcUse == "yes"
}

func connectorLocalRCCommand(r *Run, config conf.ServerConfig) string {
	if !connectorLocalRCEnabled(r, config) {
		return ""
	}
	return BuildInteractiveLocalRCShellCommand(config.LocalRcPath, config.LocalRcDecodeCmd, config.LocalRcCompress, config.LocalRcUncompressCmd)
}

func connectorPlanSupportsLocalRC(plan providerapi.ConnectorPlan, connectorName string) bool {
	if connectorManagedSSHRuntime(plan, connectorName) {
		return true
	}
	if connectorName != "aws-ssm" {
		return false
	}
	if detailString(plan.Details, "shell_runtime") != "native" {
		return false
	}
	return detailString(plan.Details, "session_action") == "start"
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
