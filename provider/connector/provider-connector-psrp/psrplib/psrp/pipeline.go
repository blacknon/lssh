package psrp

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type PipelineState int

const (
	PipelineStateNotStarted   PipelineState = 0
	PipelineStateRunning      PipelineState = 1
	PipelineStateStopping     PipelineState = 2
	PipelineStateStopped      PipelineState = 3
	PipelineStateCompleted    PipelineState = 4
	PipelineStateFailed       PipelineState = 5
	PipelineStateDisconnected PipelineState = 6
)

type PipelineStateInfo struct {
	State          PipelineState
	StateName      string
	ErrorRecordXML string
}

func BuildCreatePipelineData(script string, noInput bool) []byte {
	noInputXML := "false"
	if noInput {
		noInputXML = "true"
	}

	return []byte(fmt.Sprintf(
		`<Obj RefId="0"><MS>`+
			`<Obj N="PowerShell" RefId="1"><MS>`+
			`<Obj N="Cmds" RefId="2"><TN RefId="0"><T>System.Collections.Generic.List`+"`1"+`[[System.Management.Automation.PSObject, System.Management.Automation, Version=1.0.0.0, Culture=neutral, PublicKeyToken=31bf3856ad364e35]]</T><T>System.Object</T></TN><LST>`+
			`<Obj RefId="3"><MS>`+
			`<S N="Cmd">%s</S>`+
			`<B N="IsScript">true</B>`+
			`<Nil N="UseLocalScope" />`+
			`<Obj N="MergeMyResult" RefId="4"><TN RefId="1"><T>System.Management.Automation.Runspaces.PipelineResultTypes</T><T>System.Enum</T><T>System.ValueType</T><T>System.Object</T></TN><ToString>None</ToString><I32>0</I32></Obj>`+
			`<Obj N="MergeToResult" RefId="5"><TNRef RefId="1" /><ToString>None</ToString><I32>0</I32></Obj>`+
			`<Obj N="MergePreviousResults" RefId="6"><TNRef RefId="1" /><ToString>None</ToString><I32>0</I32></Obj>`+
			`<Obj N="MergeError" RefId="7"><TNRef RefId="1" /><ToString>None</ToString><I32>0</I32></Obj>`+
			`<Obj N="MergeWarning" RefId="8"><TNRef RefId="1" /><ToString>None</ToString><I32>0</I32></Obj>`+
			`<Obj N="MergeVerbose" RefId="9"><TNRef RefId="1" /><ToString>None</ToString><I32>0</I32></Obj>`+
			`<Obj N="MergeDebug" RefId="10"><TNRef RefId="1" /><ToString>None</ToString><I32>0</I32></Obj>`+
			`<Obj N="Args" RefId="11"><TNRef RefId="0" /><LST /></Obj>`+
			`</MS></Obj>`+
			`</LST></Obj>`+
			`<B N="IsNested">false</B>`+
			`<S N="History"></S>`+
			`<Nil N="RedirectShellErrorOutputPipe" />`+
			`<Nil N="ExtraCmds" />`+
			`</MS></Obj>`+
			`<B N="NoInput">%s</B>`+
			`<Obj N="ApartmentState" RefId="12"><TN RefId="2"><T>System.Threading.ApartmentState</T><T>System.Enum</T><T>System.ValueType</T><T>System.Object</T></TN><ToString>Unknown</ToString><I32>2</I32></Obj>`+
			`<Obj N="RemoteStreamOptions" RefId="13"><TN RefId="3"><T>System.Management.Automation.RemoteStreamOptions</T><T>System.Enum</T><T>System.ValueType</T><T>System.Object</T></TN><ToString>AddInvocationInfo</ToString><I32>15</I32></Obj>`+
			`<B N="AddToHistory">false</B>`+
			`<Obj N="HostInfo" RefId="14"><MS><B N="_isHostNull">true</B><B N="_isHostUINull">true</B><B N="_isHostRawUINull">true</B><B N="_useRunspaceHost">true</B></MS></Obj>`+
			`<Nil N="PowerShellInput" />`+
			`</MS></Obj>`,
		xmlEscape(script),
		noInputXML,
	))
}

func BuildCreatePipelineMessage(runspacePoolID, pipelineID uuid.UUID, script string, noInput bool) Message {
	return Message{
		Destination:    DestinationServer,
		Type:           MessageCreatePipeline,
		RunspacePoolID: runspacePoolID,
		PipelineID:     pipelineID,
		Data:           BuildCreatePipelineData(script, noInput),
	}
}

func ParsePipelineStateData(raw []byte) (PipelineStateInfo, error) {
	var envelope pipelineStateEnvelope
	if err := xml.Unmarshal(raw, &envelope); err != nil {
		return PipelineStateInfo{}, err
	}

	var stateNode *pipelineStateNode
	var errorNode *pipelineStateNode
	for _, node := range envelope.MS.Nodes {
		switch node.Name {
		case "PipelineState":
			nodeCopy := node
			stateNode = &nodeCopy
		case "ExceptionAsErrorRecord":
			nodeCopy := node
			errorNode = &nodeCopy
		}
	}

	if stateNode == nil || strings.TrimSpace(stateNode.Value) == "" {
		return PipelineStateInfo{}, fmt.Errorf("psrp pipeline state missing PipelineState")
	}

	state, err := parsePipelineState(strings.TrimSpace(stateNode.Value))
	if err != nil {
		return PipelineStateInfo{}, err
	}

	errorXML := ""
	if errorNode != nil {
		errorXML = strings.TrimSpace(errorNode.InnerXML)
	}

	return PipelineStateInfo{
		State:          state,
		StateName:      pipelineStateName(state, strings.TrimSpace(stateNode.ToString)),
		ErrorRecordXML: errorXML,
	}, nil
}

func (s PipelineState) Terminal() bool {
	switch s {
	case PipelineStateStopped, PipelineStateCompleted, PipelineStateFailed:
		return true
	default:
		return false
	}
}

func parsePipelineState(raw string) (PipelineState, error) {
	switch strings.TrimSpace(raw) {
	case "NotStarted":
		return PipelineStateNotStarted, nil
	case "Running":
		return PipelineStateRunning, nil
	case "Stopping":
		return PipelineStateStopping, nil
	case "Stopped":
		return PipelineStateStopped, nil
	case "Completed":
		return PipelineStateCompleted, nil
	case "Failed":
		return PipelineStateFailed, nil
	case "Disconnected":
		return PipelineStateDisconnected, nil
	case "0":
		return PipelineStateNotStarted, nil
	case "1":
		return PipelineStateRunning, nil
	case "2":
		return PipelineStateStopping, nil
	case "3":
		return PipelineStateStopped, nil
	case "4":
		return PipelineStateCompleted, nil
	case "5":
		return PipelineStateFailed, nil
	case "6":
		return PipelineStateDisconnected, nil
	default:
		return 0, fmt.Errorf("unknown psrp pipeline state %q", raw)
	}
}

func pipelineStateName(state PipelineState, current string) string {
	if current != "" {
		return current
	}
	switch state {
	case PipelineStateNotStarted:
		return "NotStarted"
	case PipelineStateRunning:
		return "Running"
	case PipelineStateStopping:
		return "Stopping"
	case PipelineStateStopped:
		return "Stopped"
	case PipelineStateCompleted:
		return "Completed"
	case PipelineStateFailed:
		return "Failed"
	case PipelineStateDisconnected:
		return "Disconnected"
	default:
		return ""
	}
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

type pipelineStateEnvelope struct {
	MS struct {
		Nodes []pipelineStateNode `xml:"Obj"`
	} `xml:"MS"`
}

type pipelineStateNode struct {
	Name     string `xml:"N,attr"`
	ToString string `xml:"ToString"`
	Value    string `xml:"I32"`
	InnerXML string `xml:",innerxml"`
}
