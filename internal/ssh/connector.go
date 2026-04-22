package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/providerapi"
	"github.com/blacknon/lssh/provider/connector/provider-connector-telnet/telnetlib"
	"github.com/blacknon/lssh/provider/connector/provider-connector-winrm/winrmlib"
	ssmconnector "github.com/blacknon/lssh/provider/mixed/provider-mixed-aws-ec2/ssmconnector"
)

func (r *Run) UsesConnector(server string) bool {
	return r.Conf.ServerUsesConnector(server)
}

func (r *Run) usesConnector(server string) bool {
	return r.UsesConnector(server)
}

func (r *Run) RunConnectorShell(server string) error {
	config := r.Conf.Server[server]
	connectorName := r.Conf.ServerConnectorName(server)
	if connectorName == "" || connectorName == "ssh" {
		return fmt.Errorf("server %q does not use an external connector", server)
	}
	if connectorHasForwarding(config) {
		return fmt.Errorf("server %q uses connector %q; ssh port forwarding is not supported", server, connectorName)
	}
	if connectorLocalRCEnabled(r, config) {
		return fmt.Errorf("server %q uses connector %q; localrc is not supported", server, connectorName)
	}

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

	switch prepared.Plan.Kind {
	case "command":
		return runCommandPlanShell(prepared.Plan)
	case "provider-managed":
		return runProviderManagedShell(prepared.Plan, planConnector)
	default:
		return fmt.Errorf("server %q connector %q returned unsupported plan kind %q", server, planConnector, prepared.Plan.Kind)
	}
}

func (r *Run) runConnectorShell(server string) error {
	return r.RunConnectorShell(server)
}

func (r *Run) RunConnectorCommand(server string, command []string, stdout, stderr io.Writer) (int, error) {
	connectorName := r.Conf.ServerConnectorName(server)
	if connectorName == "" || connectorName == "ssh" {
		return 0, fmt.Errorf("server %q does not use an external connector", server)
	}

	prepared, err := r.Conf.PrepareConnector(server, providerapi.ConnectorOperation{
		Name:    "exec",
		Command: append([]string(nil), command...),
	})
	if err != nil {
		return 0, err
	}
	planConnector := connectorNameFromPlan(prepared.Plan, connectorName)
	if !prepared.Supported {
		return 0, fmt.Errorf("server %q connector %q: %v", server, planConnector, prepared.Plan.Details["reason"])
	}

	switch prepared.Plan.Kind {
	case "command":
		return runCommandPlanExec(prepared.Plan, stdout, stderr)
	case "provider-managed":
		return runProviderManagedExec(prepared.Plan, planConnector, stdout, stderr)
	default:
		return 0, fmt.Errorf("server %q connector %q returned unsupported plan kind %q", server, planConnector, prepared.Plan.Kind)
	}
}

func (r *Run) runConnectorCommand(server string, stdout, stderr io.Writer) (int, error) {
	return r.RunConnectorCommand(server, r.ExecCmd, stdout, stderr)
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

func runProviderManagedShell(plan providerapi.ConnectorPlan, connectorName string) error {
	switch connectorName {
	case "aws-ssm":
		shellConfig, err := ssmconnector.ShellConfigFromPlan(plan)
		if err != nil {
			return err
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

func runProviderManagedExec(plan providerapi.ConnectorPlan, connectorName string, stdout, stderr io.Writer) (int, error) {
	switch connectorName {
	case "aws-ssm":
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

func runCommandPlanExec(plan providerapi.ConnectorPlan, stdout, stderr io.Writer) (int, error) {
	if strings.TrimSpace(plan.Program) == "" {
		return 0, fmt.Errorf("connector command plan is missing program")
	}

	cmd := exec.CommandContext(context.Background(), plan.Program, plan.Args...)
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

func mergedCommandPlanEnv(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}

	result := append([]string{}, os.Environ()...)
	for key, value := range env {
		result = append(result, key+"="+value)
	}
	return result
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
