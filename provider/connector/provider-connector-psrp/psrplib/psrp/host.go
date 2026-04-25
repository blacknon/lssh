package psrp

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type HostCall struct {
	CallID           int
	MethodIdentifier string
	MethodName       string
	Parameters       []string
}

func ParseHostCallData(raw []byte) (HostCall, error) {
	var envelope hostCallEnvelope
	if err := xml.Unmarshal(raw, &envelope); err != nil {
		return HostCall{}, err
	}

	callID := strings.TrimSpace(envelope.MS.CallID)
	methodID := ""
	parameters := []string{}
	for _, node := range envelope.MS.Nodes {
		switch node.Name {
		case "mi":
			methodID = strings.TrimSpace(node.ToString)
		case "mp":
			parameters = append(parameters, renderSerializedValues(node.RawXML)...)
		}
	}
	if callID == "" || methodID == "" {
		return HostCall{}, fmt.Errorf("psrp host call missing ci/mi fields")
	}

	return HostCall{
		CallID:           parseIntegerDefault(callID, 0),
		MethodIdentifier: methodID,
		MethodName:       hostMethodName(methodID),
		Parameters:       parameters,
	}, nil
}

func BuildPipelineHostResponseMessage(runspacePoolID, pipelineID uuid.UUID, callID int, resultXML string, errorXML string) Message {
	return BuildHostResponseMessage(MessagePipelineHostCall, runspacePoolID, pipelineID, callID, resultXML, errorXML)
}

func BuildRunspacePoolHostResponseMessage(runspacePoolID uuid.UUID, callID int, resultXML string, errorXML string) Message {
	return BuildHostResponseMessage(MessageRunspacePoolHostCall, runspacePoolID, uuid.Nil, callID, resultXML, errorXML)
}

func BuildHostResponseMessage(messageType MessageType, runspacePoolID, pipelineID uuid.UUID, callID int, resultXML string, errorXML string) Message {
	body := buildHostResponseXML(callID, resultXML, errorXML)
	responseType := MessagePipelineHostResponse
	if messageType == MessageRunspacePoolHostCall {
		responseType = MessageRunspacePoolHostResp
		pipelineID = uuid.Nil
	}
	return Message{
		Destination:    DestinationServer,
		Type:           responseType,
		RunspacePoolID: runspacePoolID,
		PipelineID:     pipelineID,
		Data:           []byte(body),
	}
}

func BuildDefaultHostResponse(runspacePoolID, pipelineID uuid.UUID, messageType MessageType, call HostCall) Message {
	resultXML := defaultHostResultXML(call)
	return BuildHostResponseMessage(messageType, runspacePoolID, pipelineID, call.CallID, resultXML, "")
}

func buildHostResponseXML(callID int, resultXML string, errorXML string) string {
	if strings.TrimSpace(resultXML) == "" {
		resultXML = `<Nil />`
	}
	if strings.TrimSpace(errorXML) == "" {
		errorXML = `<Nil />`
	}
	return fmt.Sprintf(`<Obj RefId="0"><MS><I64 N="ci">%d</I64><Obj N="mr">%s</Obj><Obj N="me">%s</Obj></MS></Obj>`, callID, resultXML, errorXML)
}

func hostMethodName(identifier string) string {
	switch identifier {
	case "Write1", "Write2", "WriteLine1", "WriteLine2":
		return "write"
	case "WriteErrorLine":
		return "write_error_line"
	case "WriteDebugLine":
		return "write_debug_line"
	case "WriteVerboseLine":
		return "write_verbose_line"
	case "WriteWarningLine":
		return "write_warning_line"
	case "WriteProgress":
		return "write_progress"
	case "Prompt":
		return "prompt"
	case "PromptForChoice":
		return "prompt_for_choice"
	case "PromptForChoiceMultipleSelection":
		return "prompt_for_choice_multiple_selection"
	case "ReadLine":
		return "read_line"
	case "ReadLineAsSecureString":
		return "read_line"
	default:
		return identifier
	}
}

func defaultHostResultXML(call HostCall) string {
	switch call.MethodName {
	case "read_line":
		return `<S></S>`
	case "prompt":
		return `<Obj RefId="0"><TN RefId="0"><T>System.Collections.Hashtable</T><T>System.Object</T></TN><DCT /></Obj>`
	case "prompt_for_choice":
		return `<I32>0</I32>`
	case "prompt_for_choice_multiple_selection":
		return `<Obj RefId="0"><TN RefId="0"><T>System.Int32[]</T><T>System.Array</T><T>System.Object</T></TN><LST><I32>0</I32></LST></Obj>`
	default:
		return ""
	}
}

func FormatHostPrompt(call HostCall) string {
	switch call.MethodName {
	case "prompt":
		if len(call.Parameters) == 0 {
			return "input requested"
		}
		return "prompt: " + strings.Join(call.Parameters, " ")
	case "prompt_for_choice":
		if len(call.Parameters) == 0 {
			return "choose one option"
		}
		return "choose one: " + strings.Join(call.Parameters, " | ")
	case "prompt_for_choice_multiple_selection":
		if len(call.Parameters) == 0 {
			return "choose one or more options"
		}
		return "choose one or more: " + strings.Join(call.Parameters, " | ")
	case "read_line":
		return "input: "
	case "write_progress":
		if len(call.Parameters) == 0 {
			return "progress update"
		}
		return "progress: " + strings.Join(call.Parameters, " ")
	default:
		return ""
	}
}

func parseIntegerDefault(raw string, def int) int {
	if strings.TrimSpace(raw) == "" {
		return def
	}
	var value int
	_, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &value)
	if err != nil {
		return def
	}
	return value
}

type hostCallEnvelope struct {
	MS struct {
		CallID string         `xml:"I64"`
		Nodes  []hostCallNode `xml:"Obj"`
	} `xml:"MS"`
}

type hostCallNode struct {
	Name     string `xml:"N,attr"`
	ToString string `xml:"ToString"`
	RawXML   []byte `xml:",innerxml"`
}

func renderSerializedValues(raw []byte) []string {
	decoder := xml.NewDecoder(strings.NewReader("<root>" + string(raw) + "</root>"))
	values := []string{}
	var current strings.Builder
	inText := false

	flush := func() {
		text := strings.TrimSpace(current.String())
		if text != "" {
			values = append(values, text)
		}
		current.Reset()
	}

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch typed := token.(type) {
		case xml.StartElement:
			switch typed.Name.Local {
			case "S", "ToString":
				inText = true
			}
		case xml.EndElement:
			switch typed.Name.Local {
			case "S", "ToString":
				inText = false
				flush()
			}
		case xml.CharData:
			if inText {
				current.WriteString(string(typed))
			}
		}
	}

	return values
}
