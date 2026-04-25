package wsman

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/psrp"
	"github.com/google/uuid"
)

const (
	actionCreateShell     = "http://schemas.xmlsoap.org/ws/2004/09/transfer/Create"
	actionDeleteShell     = "http://schemas.xmlsoap.org/ws/2004/09/transfer/Delete"
	actionCommand         = "http://schemas.microsoft.com/wbem/wsman/1/windows/shell/Command"
	actionReceive         = "http://schemas.microsoft.com/wbem/wsman/1/windows/shell/Receive"
	actionSend            = "http://schemas.microsoft.com/wbem/wsman/1/windows/shell/Send"
	actionSignal          = "http://schemas.microsoft.com/wbem/wsman/1/windows/shell/Signal"
	resourceURIPowerShell = "http://schemas.microsoft.com/powershell/Microsoft.PowerShell"
	resourceURICmdShell   = "http://schemas.microsoft.com/wbem/wsman/1/windows/shell/cmd"
	commandStateDone      = "http://schemas.microsoft.com/wbem/wsman/1/windows/shell/CommandState/Done"
)

type Shell struct {
	ID          string
	ResourceURI string
}

type Command struct {
	ID       string
	ShellID  string
	Streams  []string
	ExitCode int
	Done     bool
}

type ReceiveResult struct {
	Stdout   []byte
	Stderr   []byte
	Done     bool
	ExitCode int
}

func (c *Client) CreateShell(ctx context.Context, powershell bool) (Shell, error) {
	resourceURI := resourceURICmdShell
	if powershell {
		resourceURI = resourceURIPowerShell
	}

	raw, err := c.Do(ctx, Header{
		Action:      actionCreateShell,
		ResourceURI: resourceURI,
		MessageID:   "uuid:" + uuid.NewString(),
	}, createShellBody())
	if err != nil {
		return Shell{}, err
	}

	response, err := parseCreateShellResponse(raw)
	if err != nil {
		return Shell{}, err
	}
	response.ResourceURI = resourceURI
	return response, nil
}

func (c *Client) CreatePSRPShell(ctx context.Context, init psrp.RunspacePoolInit) (Shell, uuid.UUID, error) {
	runspacePoolID := uuid.New()
	creationXML, err := psrp.BuildOpenRunspacePoolCreationXML(runspacePoolID, init)
	if err != nil {
		return Shell{}, uuid.Nil, err
	}

	var response Shell
	for _, protocolVersion := range createShellProtocolVersions() {
		response, err = c.createPSRPShell(ctx, creationXML, protocolVersion)
		if err == nil {
			response.ResourceURI = resourceURIPowerShell
			return response, runspacePoolID, nil
		}
		if !isInvalidOptionSetError(err) {
			return Shell{}, uuid.Nil, err
		}
	}

	response, err = c.createPSRPShell(ctx, creationXML, "")
	if err == nil {
		response.ResourceURI = resourceURIPowerShell
		return response, runspacePoolID, nil
	}

	return Shell{}, uuid.Nil, err
}

func (c *Client) createPSRPShell(ctx context.Context, creationXML []byte, protocolVersion string) (Shell, error) {
	options := []Option(nil)
	if strings.TrimSpace(protocolVersion) != "" {
		options = []Option{{
			Name:       "ProtocolVersion",
			Value:      protocolVersion,
			MustComply: true,
		}}
	}

	raw, err := c.Do(ctx, Header{
		Action:      actionCreateShell,
		ResourceURI: resourceURIPowerShell,
		MessageID:   "uuid:" + uuid.NewString(),
		Options:     options,
	}, createPSRPShellBody(c.Endpoint, creationXML))
	if err != nil {
		return Shell{}, err
	}

	return parseCreateShellResponse(raw)
}

func createShellProtocolVersions() []string {
	return []string{psrp.CreateShellProtocolVersion, "2.1"}
}

func isInvalidOptionSetError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "InvalidOptions") || strings.Contains(message, "invalid WS-Management OptionSet")
}

func (c *Client) DeleteShell(ctx context.Context, shell Shell) error {
	_, err := c.Do(ctx, Header{
		Action:      actionDeleteShell,
		ResourceURI: shell.ResourceURI,
		MessageID:   "uuid:" + uuid.NewString(),
		Selectors: []Selector{
			{Name: "ShellId", Value: shell.ID},
		},
	}, "")
	return err
}

func (c *Client) Execute(ctx context.Context, shell Shell, command string, arguments ...string) (Command, error) {
	raw, err := c.Do(ctx, Header{
		Action:      actionCommand,
		ResourceURI: shell.ResourceURI,
		MessageID:   "uuid:" + uuid.NewString(),
		Selectors: []Selector{
			{Name: "ShellId", Value: shell.ID},
		},
	}, executeCommandBody(command, arguments))
	if err != nil {
		return Command{}, err
	}

	commandResp, err := parseExecuteCommandResponse(raw)
	if err != nil {
		return Command{}, err
	}
	commandResp.ShellID = shell.ID
	commandResp.Streams = []string{"stdout", "stderr"}
	return commandResp, nil
}

func (c *Client) StartPipeline(ctx context.Context, shell Shell, pipelineID uuid.UUID, message psrp.Message) (Command, error) {
	fragments := fragmentPipelineMessage(message, 0)
	firstFragment, continuationPayloads, err := buildPSRPCommandPayloads(fragments)
	if err != nil {
		return Command{}, err
	}

	raw, err := c.Do(ctx, Header{
		Action:      actionCommand,
		ResourceURI: shell.ResourceURI,
		MessageID:   "uuid:" + uuid.NewString(),
		Selectors: []Selector{
			{Name: "ShellId", Value: shell.ID},
		},
	}, executePSRPCommandBody(firstFragment))
	if err != nil {
		return Command{}, err
	}

	commandResp, err := parseExecuteCommandResponse(raw)
	if err != nil {
		return Command{}, err
	}
	commandResp.ShellID = shell.ID
	commandResp.Streams = []string{"stdout"}

	for _, payload := range continuationPayloads {
		if err := c.Send(ctx, shell, commandResp, payload, false); err != nil {
			return Command{}, err
		}
	}
	return commandResp, nil
}

func (c *Client) Receive(ctx context.Context, shell Shell, command Command) (ReceiveResult, error) {
	raw, err := c.Do(ctx, Header{
		Action:      actionReceive,
		ResourceURI: shell.ResourceURI,
		MessageID:   "uuid:" + uuid.NewString(),
		Selectors: []Selector{
			{Name: "ShellId", Value: shell.ID},
		},
	}, receiveBody(command.ID, command.Streams))
	if err != nil {
		return ReceiveResult{}, err
	}
	return parseReceiveResponse(raw)
}

func (c *Client) ReceiveShell(ctx context.Context, shell Shell, streams []string) (ReceiveResult, error) {
	raw, err := c.Do(ctx, Header{
		Action:      actionReceive,
		ResourceURI: shell.ResourceURI,
		MessageID:   "uuid:" + uuid.NewString(),
		Selectors: []Selector{
			{Name: "ShellId", Value: shell.ID},
		},
	}, receiveBody("", streams))
	if err != nil {
		return ReceiveResult{}, err
	}
	return parseReceiveResponse(raw)
}

func (c *Client) Send(ctx context.Context, shell Shell, command Command, input []byte, eof bool) error {
	_, err := c.Do(ctx, Header{
		Action:      actionSend,
		ResourceURI: shell.ResourceURI,
		MessageID:   "uuid:" + uuid.NewString(),
		Selectors: []Selector{
			{Name: "ShellId", Value: shell.ID},
		},
	}, sendBody("stdin", command.ID, input, eof))
	return err
}

func (c *Client) SendShell(ctx context.Context, shell Shell, stream string, input []byte, eof bool) error {
	_, err := c.Do(ctx, Header{
		Action:      actionSend,
		ResourceURI: shell.ResourceURI,
		MessageID:   "uuid:" + uuid.NewString(),
		Selectors: []Selector{
			{Name: "ShellId", Value: shell.ID},
		},
	}, sendBody(stream, "", input, eof))
	return err
}

func (c *Client) SignalTerminate(ctx context.Context, shell Shell, command Command) error {
	_, err := c.Do(ctx, Header{
		Action:      actionSignal,
		ResourceURI: shell.ResourceURI,
		MessageID:   "uuid:" + uuid.NewString(),
		Selectors: []Selector{
			{Name: "ShellId", Value: shell.ID},
		},
	}, signalBody(command.ID))
	return err
}

func createShellBody() string {
	return `<r:Shell><r:InputStreams>stdin</r:InputStreams><r:OutputStreams>stdout stderr</r:OutputStreams></r:Shell>`
}

func createPSRPShellBody(_ Endpoint, creationXML []byte) string {
	shellID := strings.ToUpper(uuid.NewString())
	return fmt.Sprintf(
		`<r:Shell ShellId="%s" Name="WinRM2"><r:InputStreams>stdin pr</r:InputStreams><r:OutputStreams>stdout</r:OutputStreams>%s</r:Shell>`,
		shellID,
		string(creationXML),
	)
}

func executeCommandBody(command string, arguments []string) string {
	var builder strings.Builder
	builder.WriteString(`<r:CommandLine>`)
	builder.WriteString(`<r:Command><![CDATA[`)
	builder.WriteString(command)
	builder.WriteString(`]]></r:Command>`)
	for _, arg := range arguments {
		builder.WriteString(`<r:Arguments><![CDATA[`)
		builder.WriteString(arg)
		builder.WriteString(`]]></r:Arguments>`)
	}
	builder.WriteString(`</r:CommandLine>`)
	return builder.String()
}

func executePSRPCommandBody(fragment psrp.Fragment) string {
	return fmt.Sprintf(
		`<r:CommandLine><r:Command>%s</r:Command></r:CommandLine>`,
		base64.StdEncoding.EncodeToString(psrp.EncodeFragment(fragment)),
	)
}

func fragmentPipelineMessage(message psrp.Message, maxBlobSize int) []psrp.Fragment {
	return psrp.FragmentMessage(1, psrp.EncodeMessage(message), maxBlobSize)
}

func buildPSRPCommandPayloads(fragments []psrp.Fragment) (psrp.Fragment, [][]byte, error) {
	if len(fragments) == 0 {
		return psrp.Fragment{}, nil, fmt.Errorf("psrp command requires at least one fragment")
	}

	first := fragments[0]
	continuations := make([][]byte, 0, len(fragments)-1)
	for _, fragment := range fragments[1:] {
		continuations = append(continuations, psrp.EncodeFragment(fragment))
	}

	return first, continuations, nil
}

func receiveBody(commandID string, streams []string) string {
	commandAttr := ""
	if commandID != "" {
		commandAttr = fmt.Sprintf(` CommandId="%s"`, commandID)
	}
	return fmt.Sprintf(`<r:Receive><r:DesiredStream%s>%s</r:DesiredStream></r:Receive>`, commandAttr, strings.Join(streams, " "))
}

func sendBody(streamName, commandID string, input []byte, eof bool) string {
	endAttr := ""
	if eof {
		endAttr = ` End="true"`
	}
	commandAttr := ""
	if commandID != "" {
		commandAttr = fmt.Sprintf(` CommandId="%s"`, commandID)
	}
	if strings.TrimSpace(streamName) == "" {
		streamName = "stdin"
	}
	return fmt.Sprintf(`<r:Send><r:Stream Name="%s"%s%s>%s</r:Stream></r:Send>`,
		streamName,
		commandAttr,
		endAttr,
		base64.StdEncoding.EncodeToString(input),
	)
}

func signalBody(commandID string) string {
	return fmt.Sprintf(`<r:Signal CommandId="%s"><r:Code>http://schemas.microsoft.com/wbem/wsman/1/windows/shell/signal/terminate</r:Code></r:Signal>`, commandID)
}

func parseCreateShellResponse(raw []byte) (Shell, error) {
	var envelope createShellEnvelope
	if err := xml.Unmarshal(raw, &envelope); err != nil {
		return Shell{}, err
	}
	if envelope.Body.ResourceCreated.SelectorSet.ShellID == "" {
		return Shell{}, fmt.Errorf("wsman create shell response missing ShellId")
	}
	return Shell{ID: strings.TrimSpace(envelope.Body.ResourceCreated.SelectorSet.ShellID)}, nil
}

func parseExecuteCommandResponse(raw []byte) (Command, error) {
	var envelope executeCommandEnvelope
	if err := xml.Unmarshal(raw, &envelope); err != nil {
		return Command{}, err
	}
	if strings.TrimSpace(envelope.Body.CommandResponse.CommandID) == "" {
		return Command{}, fmt.Errorf("wsman command response missing CommandId")
	}
	return Command{ID: strings.TrimSpace(envelope.Body.CommandResponse.CommandID)}, nil
}

func parseReceiveResponse(raw []byte) (ReceiveResult, error) {
	var envelope receiveEnvelope
	if err := xml.Unmarshal(raw, &envelope); err != nil {
		return ReceiveResult{}, err
	}

	result := ReceiveResult{}
	for _, stream := range envelope.Body.ReceiveResponse.Streams {
		payload, err := base64.StdEncoding.DecodeString(strings.TrimSpace(stream.Value))
		if err != nil {
			return ReceiveResult{}, err
		}
		switch stream.Name {
		case "stdout":
			result.Stdout = append(result.Stdout, payload...)
		case "stderr":
			result.Stderr = append(result.Stderr, payload...)
		}
	}

	state := strings.TrimSpace(envelope.Body.ReceiveResponse.CommandState.State)
	if state == commandStateDone {
		result.Done = true
	}
	if exit := strings.TrimSpace(envelope.Body.ReceiveResponse.CommandState.ExitCode); exit != "" {
		code, err := strconv.Atoi(exit)
		if err != nil {
			return ReceiveResult{}, err
		}
		result.ExitCode = code
	}
	return result, nil
}

type createShellEnvelope struct {
	Body struct {
		ResourceCreated struct {
			SelectorSet struct {
				ShellID string `xml:"Selector"`
			} `xml:"ReferenceParameters>SelectorSet"`
		} `xml:"ResourceCreated"`
	} `xml:"Body"`
}

type executeCommandEnvelope struct {
	Body struct {
		CommandResponse struct {
			CommandID string `xml:"CommandId"`
		} `xml:"CommandResponse"`
	} `xml:"Body"`
}

type receiveEnvelope struct {
	Body struct {
		ReceiveResponse struct {
			Streams []struct {
				Name  string `xml:"Name,attr"`
				Value string `xml:",chardata"`
			} `xml:"Stream"`
			CommandState struct {
				State    string `xml:"State,attr"`
				ExitCode string `xml:"ExitCode"`
			} `xml:"CommandState"`
		} `xml:"ReceiveResponse"`
	} `xml:"Body"`
}
