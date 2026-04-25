package psrplib

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/psrp"
	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/wsman"
)

const (
	helperCommand = "__psrp_helper"
	helperShell   = "shell"
	helperExec    = "exec"
)

func IsHelperInvocation(args []string) bool {
	return len(args) > 0 && args[0] == helperCommand
}

func RunHelper(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		_, _ = io.WriteString(stderr, "psrp helper requires an operation\n")
		return 2
	}

	cfg, err := ConfigFromEnv()
	if err != nil {
		_, _ = io.WriteString(stderr, err.Error()+"\n")
		return 1
	}

	switch args[1] {
	case helperShell:
		if err := runLibraryShell(cfg, stdin, stdout, stderr); err != nil {
			_, _ = io.WriteString(stderr, err.Error()+"\n")
			return 1
		}
		return 0
	case helperExec:
		commandLine := os.Getenv(envPowerShellCommand)
		if commandLine == "" {
			_, _ = io.WriteString(stderr, "psrp helper exec requires command metadata\n")
			return 2
		}
		exitCode, err := runLibraryExec(cfg, commandLine, stdin, stdout, stderr)
		if err != nil {
			_, _ = io.WriteString(stderr, err.Error()+"\n")
			return 1
		}
		return exitCode
	default:
		_, _ = io.WriteString(stderr, fmt.Sprintf("unsupported psrp helper operation %q\n", args[1]))
		return 2
	}
}

func runLibraryShell(cfg Config, stdin io.Reader, stdout, stderr io.Writer) error {
	client := newWSManClient(cfg)
	ctx := context.Background()

	pool, _, err := OpenRunspacePool(ctx, client, psrp.DefaultRunspacePoolInit())
	if err != nil {
		return err
	}
	defer func() { _ = client.DeleteShell(ctx, pool.Shell) }()

	pipeline, err := StartPipeline(ctx, client, pool, buildInteractiveShellScript(), false)
	if err != nil {
		return err
	}

	return streamLibraryPipeline(ctx, client, pool, pipeline, stdin, stdout, stderr)
}

func runLibraryExec(cfg Config, commandLine string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	client := newWSManClient(cfg)
	ctx := context.Background()
	broker := newLineBroker(stdin)

	pool, _, err := OpenRunspacePool(ctx, client, psrp.DefaultRunspacePoolInit())
	if err != nil {
		return 1, err
	}
	defer func() { _ = client.DeleteShell(ctx, pool.Shell) }()

	pipeline, err := StartPipeline(ctx, client, pool, commandLine, true)
	if err != nil {
		return 1, err
	}

	result, err := collectPipelineResult(ctx, client, pool, pipeline, broker, stdout, stderr)
	if err != nil {
		return 1, err
	}
	if result.State.State == psrp.PipelineStateFailed {
		return 1, fmt.Errorf("psrp pipeline failed")
	}
	if result.State.State == psrp.PipelineStateStopped {
		return 1, fmt.Errorf("psrp pipeline stopped")
	}
	return 0, nil
}

func streamLibraryCommand(ctx context.Context, client *wsman.Client, shell wsman.Shell, command wsman.Command, stdin io.Reader, stdout, stderr io.Writer) error {
	errCh := make(chan error, 2)
	done := make(chan struct{})

	if stdin != nil {
		go func() {
			if err := copyLibraryInput(ctx, client, shell, command, stdin); err != nil {
				errCh <- err
				return
			}
			close(done)
		}()
	}

	var streamWG sync.WaitGroup
	streamWG.Add(1)
	go func() {
		defer streamWG.Done()
		for {
			result, err := client.Receive(ctx, shell, command)
			if err != nil {
				errCh <- err
				return
			}
			if len(result.Stdout) > 0 && stdout != nil {
				_, _ = stdout.Write(result.Stdout)
			}
			if len(result.Stderr) > 0 && stderr != nil {
				_, _ = stderr.Write(result.Stderr)
			}
			if result.Done {
				command.Done = true
				command.ExitCode = result.ExitCode
				return
			}
		}
	}()

	streamWG.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func copyLibraryInput(ctx context.Context, client *wsman.Client, shell wsman.Shell, command wsman.Command, stdin io.Reader) error {
	buf := make([]byte, 1024)
	for {
		n, err := stdin.Read(buf)
		if n > 0 {
			if sendErr := client.Send(ctx, shell, command, buf[:n], false); sendErr != nil {
				return sendErr
			}
		}
		if err != nil {
			if err == io.EOF {
				return client.Send(ctx, shell, command, nil, true)
			}
			return err
		}
	}
}

func streamLibraryPipeline(ctx context.Context, client *wsman.Client, pool RunspacePool, pipeline Pipeline, stdin io.Reader, stdout, stderr io.Writer) error {
	broker := newLineBroker(stdin)
	errCh := make(chan error, 2)

	go func() {
		if err := copyPipelineInput(ctx, client, pool, pipeline, broker); err != nil {
			errCh <- err
		}
	}()

	go func() {
		if err := copyPipelineOutput(ctx, client, pool, pipeline, broker, stdout, stderr); err != nil {
			errCh <- err
		}
	}()

	err := <-errCh
	if err == io.EOF {
		return nil
	}
	return err
}

func copyPipelineInput(ctx context.Context, client *wsman.Client, pool RunspacePool, pipeline Pipeline, broker *lineBroker) error {
	for {
		line, done, err := broker.Next(ctx)
		if err != nil {
			return err
		}
		if done {
			return SendPipelineInput(ctx, client, pool.Shell, pool.ID, pipeline.ID, "", true)
		}
		if err := SendPipelineInput(ctx, client, pool.Shell, pool.ID, pipeline.ID, line, false); err != nil {
			return err
		}
	}
}

func copyPipelineOutput(ctx context.Context, client *wsman.Client, pool RunspacePool, pipeline Pipeline, broker *lineBroker, stdout, stderr io.Writer) error {
	_, err := collectPipelineResult(ctx, client, pool, pipeline, broker, stdout, stderr)
	if err != nil {
		return err
	}
	return io.EOF
}

func buildHostResponseFromInput(ctx context.Context, broker *lineBroker, pool RunspacePool, pipeline Pipeline, messageType psrp.MessageType, hostCall psrp.HostCall) (psrp.Message, error) {
	switch hostCall.MethodName {
	case "read_line":
		line, _, err := broker.Next(ctx)
		if err != nil && err != io.EOF {
			return psrp.Message{}, err
		}
		return psrp.BuildHostResponseMessage(messageType, pool.ID, pipeline.ID, hostCall.CallID, "<S>"+xmlEscapeHostText(line)+"</S>", ""), nil
	case "prompt":
		line, _, err := broker.Next(ctx)
		if err != nil && err != io.EOF {
			return psrp.Message{}, err
		}
		return psrp.BuildHostResponseMessage(messageType, pool.ID, pipeline.ID, hostCall.CallID, buildPromptResponseXML(line), ""), nil
	case "prompt_for_choice":
		line, _, err := broker.Next(ctx)
		if err != nil && err != io.EOF {
			return psrp.Message{}, err
		}
		return psrp.BuildHostResponseMessage(messageType, pool.ID, pipeline.ID, hostCall.CallID, buildChoiceResponseXML(line), ""), nil
	case "prompt_for_choice_multiple_selection":
		line, _, err := broker.Next(ctx)
		if err != nil && err != io.EOF {
			return psrp.Message{}, err
		}
		return psrp.BuildHostResponseMessage(messageType, pool.ID, pipeline.ID, hostCall.CallID, buildMultiChoiceResponseXML(line), ""), nil
	default:
		return psrp.BuildDefaultHostResponse(pool.ID, pipeline.ID, messageType, hostCall), nil
	}
}

func collectPipelineResult(ctx context.Context, client *wsman.Client, pool RunspacePool, pipeline Pipeline, broker *lineBroker, stdout, stderr io.Writer) (PipelineResult, error) {
	result := PipelineResult{
		CommandID: pipeline.Command.ID,
	}
	decoder := &psrp.MessageStreamDecoder{}

	for {
		received, err := client.Receive(ctx, pool.Shell, pipeline.Command)
		if err != nil {
			return PipelineResult{}, err
		}
		if len(received.Stdout) > 0 {
			result.RawStdout = append(result.RawStdout, append([]byte(nil), received.Stdout...))
			messages, err := decoder.Push(received.Stdout)
			if err != nil {
				return PipelineResult{}, err
			}
			result.Messages = append(result.Messages, messages...)
			for _, message := range messages {
				if err := handlePipelineMessage(ctx, client, pool, pipeline, broker, message, &result, stdout, stderr); err != nil {
					if err == io.EOF {
						return result, nil
					}
					return PipelineResult{}, err
				}
			}
		}
		if len(received.Stderr) > 0 {
			result.RawStderr = append(result.RawStderr, append([]byte(nil), received.Stderr...))
			if stderr != nil {
				_, _ = stderr.Write(received.Stderr)
			}
		}
		if received.Done {
			return result, nil
		}
	}
}

func handlePipelineMessage(ctx context.Context, client *wsman.Client, pool RunspacePool, pipeline Pipeline, broker *lineBroker, message psrp.Message, result *PipelineResult, stdout, stderr io.Writer) error {
	switch message.Type {
	case psrp.MessagePipelineOutput:
		if message.PipelineID != pipeline.ID {
			return nil
		}
		result.Outputs = append(result.Outputs, append([]byte(nil), message.Data...))
		if stdout != nil {
			_, _ = io.WriteString(stdout, psrp.RenderSerializedText(message.Data))
			_, _ = io.WriteString(stdout, "\n")
		}
	case psrp.MessageErrorRecord:
		if message.PipelineID != pipeline.ID {
			return nil
		}
		result.Errors = append(result.Errors, append([]byte(nil), message.Data...))
		if stderr != nil {
			_, _ = io.WriteString(stderr, psrp.RenderSerializedText(message.Data))
			_, _ = io.WriteString(stderr, "\n")
		}
	case psrp.MessagePipelineHostCall:
		if message.PipelineID != pipeline.ID {
			return nil
		}
		hostCall, err := psrp.ParseHostCallData(message.Data)
		if err != nil {
			return err
		}
		renderHostCall(stdout, stderr, hostCall)
		response, err := buildHostResponseFromInput(ctx, broker, pool, pipeline, message.Type, hostCall)
		if err != nil {
			return err
		}
		return SendPSRPMessages(ctx, client, pool.Shell, "stdin", []psrp.Message{response})
	case psrp.MessageRunspacePoolHostCall:
		hostCall, err := psrp.ParseHostCallData(message.Data)
		if err != nil {
			return err
		}
		renderHostCall(stdout, stderr, hostCall)
		response, err := buildHostResponseFromInput(ctx, broker, pool, pipeline, message.Type, hostCall)
		if err != nil {
			return err
		}
		return SendPSRPMessages(ctx, client, pool.Shell, "stdin", []psrp.Message{response})
	case psrp.MessagePipelineState:
		if message.PipelineID != pipeline.ID {
			return nil
		}
		state, err := psrp.ParsePipelineStateData(message.Data)
		if err != nil {
			return err
		}
		result.State = state
		if state.State.Terminal() {
			return io.EOF
		}
	}
	return nil
}

func xmlEscapeHostText(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func buildPromptResponseXML(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return `<Obj RefId="0"><TN RefId="0"><T>System.Collections.Hashtable</T><T>System.Object</T></TN><DCT /></Obj>`
	}

	var builder strings.Builder
	builder.WriteString(`<Obj RefId="0"><TN RefId="0"><T>System.Collections.Hashtable</T><T>System.Object</T></TN><DCT>`)
	for _, pair := range strings.Split(line, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		builder.WriteString(`<En><S N="Key">`)
		builder.WriteString(xmlEscapeHostText(key))
		builder.WriteString(`</S><S N="Value">`)
		builder.WriteString(xmlEscapeHostText(value))
		builder.WriteString(`</S></En>`)
	}
	builder.WriteString(`</DCT></Obj>`)
	return builder.String()
}

func buildChoiceResponseXML(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return `<I32>0</I32>`
	}
	return `<I32>` + xmlEscapeHostText(line) + `</I32>`
}

func buildMultiChoiceResponseXML(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return `<Obj RefId="0"><TN RefId="0"><T>System.Int32[]</T><T>System.Array</T><T>System.Object</T></TN><LST><I32>0</I32></LST></Obj>`
	}

	var builder strings.Builder
	builder.WriteString(`<Obj RefId="0"><TN RefId="0"><T>System.Int32[]</T><T>System.Array</T><T>System.Object</T></TN><LST>`)
	for _, part := range strings.Split(line, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		builder.WriteString(`<I32>`)
		builder.WriteString(xmlEscapeHostText(part))
		builder.WriteString(`</I32>`)
	}
	builder.WriteString(`</LST></Obj>`)
	return builder.String()
}

func renderHostCall(stdout, stderr io.Writer, hostCall psrp.HostCall) {
	target := stdout
	if hostCall.MethodName == "write_error_line" || hostCall.MethodName == "write_warning_line" {
		target = stderr
	}
	if target == nil {
		return
	}

	prompt := psrp.FormatHostPrompt(hostCall)
	if prompt != "" {
		_, _ = io.WriteString(target, prompt)
		if !strings.HasSuffix(prompt, " ") {
			_, _ = io.WriteString(target, "\n")
		}
	}

	for _, parameter := range hostCall.Parameters {
		if prompt != "" && (hostCall.MethodName == "prompt" || hostCall.MethodName == "prompt_for_choice" || hostCall.MethodName == "prompt_for_choice_multiple_selection" || hostCall.MethodName == "read_line") {
			break
		}
		if strings.TrimSpace(parameter) == "" {
			continue
		}
		_, _ = io.WriteString(target, parameter)
		_, _ = io.WriteString(target, "\n")
	}
}

func buildInteractiveShellScript() string {
	return strings.Join([]string{
		"$ErrorActionPreference = 'Continue'",
		"$promptText = { \"PS \" + (Get-Location) + \"> \" }",
		"Write-Output (& $promptText)",
		"process {",
		"  if ($_ -eq $null) { return }",
		"  $command = [string]$_",
		"  if ($command.Length -eq 0) { Write-Output (& $promptText); return }",
		"  try {",
		"    $output = Invoke-Expression $command 2>&1 | Out-String",
		"    if ($output) { Write-Output $output.TrimEnd(\"`r\", \"`n\") }",
		"  } catch {",
		"    Write-Error $_",
		"  }",
		"  Write-Output (& $promptText)",
		"}",
	}, " ")
}

func newWSManClient(cfg Config) *wsman.Client {
	scheme := "http"
	if cfg.HTTPS {
		scheme = "https"
	}
	return &wsman.Client{
		Endpoint: wsman.Endpoint{
			Scheme: scheme,
			Host:   cfg.Host,
			Port:   cfg.Port,
			Path:   "/wsman",
		},
		Username:    cfg.User,
		Password:    cfg.Password,
		InsecureTLS: cfg.Insecure,
	}
}
