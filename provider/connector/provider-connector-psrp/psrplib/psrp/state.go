package psrp

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

type RunspacePoolState int

const (
	RunspacePoolStateBeforeOpen           RunspacePoolState = 0
	RunspacePoolStateOpening              RunspacePoolState = 1
	RunspacePoolStateOpened               RunspacePoolState = 2
	RunspacePoolStateClosed               RunspacePoolState = 3
	RunspacePoolStateClosing              RunspacePoolState = 4
	RunspacePoolStateBroken               RunspacePoolState = 5
	RunspacePoolStateNegotiationSucceeded RunspacePoolState = 6
	RunspacePoolStateNegotiationFailed    RunspacePoolState = 7
)

type RunspacePoolStateInfo struct {
	State          RunspacePoolState
	StateName      string
	ErrorRecordXML string
}

func ParseRunspacePoolStateData(raw []byte) (RunspacePoolStateInfo, error) {
	var envelope runspacePoolStateEnvelope
	if err := xml.Unmarshal(raw, &envelope); err != nil {
		return RunspacePoolStateInfo{}, err
	}

	var stateNode *runspacePoolStateNode
	var errorNode *runspacePoolStateNode
	for _, node := range envelope.MS.Nodes {
		switch node.Name {
		case "RunspaceState":
			nodeCopy := node
			stateNode = &nodeCopy
		case "ExceptionAsErrorRecord":
			nodeCopy := node
			errorNode = &nodeCopy
		}
	}

	if stateNode == nil || strings.TrimSpace(stateNode.Value) == "" {
		return RunspacePoolStateInfo{}, fmt.Errorf("psrp runspace pool state missing RunspaceState")
	}

	state, err := parseRunspacePoolState(stateNode.Value)
	if err != nil {
		return RunspacePoolStateInfo{}, err
	}

	errorXML := ""
	if errorNode != nil {
		errorXML = strings.TrimSpace(errorNode.InnerXML)
	}

	return RunspacePoolStateInfo{
		State:          state,
		StateName:      strings.TrimSpace(stateNode.ToString),
		ErrorRecordXML: errorXML,
	}, nil
}

func parseRunspacePoolState(raw string) (RunspacePoolState, error) {
	trimmed := strings.TrimSpace(raw)
	switch trimmed {
	case "BeforeOpen":
		return RunspacePoolStateBeforeOpen, nil
	case "Opening":
		return RunspacePoolStateOpening, nil
	case "Opened":
		return RunspacePoolStateOpened, nil
	case "Closed":
		return RunspacePoolStateClosed, nil
	case "Closing":
		return RunspacePoolStateClosing, nil
	case "Broken":
		return RunspacePoolStateBroken, nil
	case "NegotiationSucceeded":
		return RunspacePoolStateNegotiationSucceeded, nil
	case "NegotiationFailed":
		return RunspacePoolStateNegotiationFailed, nil
	default:
		numeric, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, fmt.Errorf("unknown psrp runspace pool state %q", raw)
		}
		switch RunspacePoolState(numeric) {
		case RunspacePoolStateBeforeOpen,
			RunspacePoolStateOpening,
			RunspacePoolStateOpened,
			RunspacePoolStateClosed,
			RunspacePoolStateClosing,
			RunspacePoolStateBroken,
			RunspacePoolStateNegotiationSucceeded,
			RunspacePoolStateNegotiationFailed:
			return RunspacePoolState(numeric), nil
		default:
			return 0, fmt.Errorf("unknown psrp runspace pool state %q", raw)
		}
	}
}

func (s RunspacePoolState) Terminal() bool {
	switch s {
	case RunspacePoolStateOpened, RunspacePoolStateClosed, RunspacePoolStateBroken, RunspacePoolStateNegotiationSucceeded, RunspacePoolStateNegotiationFailed:
		return true
	default:
		return false
	}
}

type runspacePoolStateEnvelope struct {
	MS struct {
		Nodes []runspacePoolStateNode `xml:"Obj"`
	} `xml:"MS"`
}

type runspacePoolStateNode struct {
	Name     string `xml:"N,attr"`
	ToString string `xml:"ToString"`
	Value    string `xml:"I32"`
	InnerXML string `xml:",innerxml"`
}
