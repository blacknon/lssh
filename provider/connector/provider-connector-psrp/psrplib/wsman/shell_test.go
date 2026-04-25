package wsman

import (
	"strings"
	"testing"

	"github.com/blacknon/lssh/provider/connector/provider-connector-psrp/psrplib/psrp"
	"github.com/google/uuid"
)

func TestParseCreateShellResponse(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:wst="http://schemas.xmlsoap.org/ws/2004/09/transfer" xmlns:wsa="http://schemas.xmlsoap.org/ws/2004/08/addressing" xmlns:wsman="http://schemas.dmtf.org/wbem/wsman/1/wsman.xsd">
  <s:Body>
    <wst:ResourceCreated>
      <wsa:ReferenceParameters>
        <wsman:SelectorSet>
          <wsman:Selector Name="ShellId">uuid:shell-id</wsman:Selector>
        </wsman:SelectorSet>
      </wsa:ReferenceParameters>
    </wst:ResourceCreated>
  </s:Body>
</s:Envelope>`)

	shell, err := parseCreateShellResponse(raw)
	if err != nil {
		t.Fatalf("parseCreateShellResponse() error = %v", err)
	}
	if shell.ID != "uuid:shell-id" {
		t.Fatalf("Shell.ID = %q, want uuid:shell-id", shell.ID)
	}
}

func TestParseExecuteCommandResponse(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:r="http://schemas.microsoft.com/wbem/wsman/1/windows/shell">
  <s:Body>
    <r:CommandResponse>
      <r:CommandId>uuid:command-id</r:CommandId>
    </r:CommandResponse>
  </s:Body>
</s:Envelope>`)

	command, err := parseExecuteCommandResponse(raw)
	if err != nil {
		t.Fatalf("parseExecuteCommandResponse() error = %v", err)
	}
	if command.ID != "uuid:command-id" {
		t.Fatalf("Command.ID = %q, want uuid:command-id", command.ID)
	}
}

func TestParseReceiveResponse(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:r="http://schemas.microsoft.com/wbem/wsman/1/windows/shell">
  <s:Body>
    <r:ReceiveResponse>
      <r:Stream Name="stdout">aGVsbG8=</r:Stream>
      <r:Stream Name="stderr">d2Fybgo=</r:Stream>
      <r:CommandState State="http://schemas.microsoft.com/wbem/wsman/1/windows/shell/CommandState/Done">
        <r:ExitCode>0</r:ExitCode>
      </r:CommandState>
    </r:ReceiveResponse>
  </s:Body>
</s:Envelope>`)

	result, err := parseReceiveResponse(raw)
	if err != nil {
		t.Fatalf("parseReceiveResponse() error = %v", err)
	}
	if string(result.Stdout) != "hello" {
		t.Fatalf("Stdout = %q, want hello", result.Stdout)
	}
	if string(result.Stderr) != "warn\n" {
		t.Fatalf("Stderr = %q, want warn\\n", result.Stderr)
	}
	if !result.Done {
		t.Fatal("Done = false, want true")
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestCreatePSRPShellBody(t *testing.T) {
	body := createPSRPShellBody(Endpoint{
		Scheme: "https",
		Host:   "windows.local",
		Port:   5986,
	}, []byte(`<creationXml xmlns="http://schemas.microsoft.com/powershell">abc</creationXml>`))

	for _, want := range []string{
		`<r:Shell ShellId="`,
		`Name="WinRM2"`,
		`<r:InputStreams>stdin pr</r:InputStreams>`,
		`<r:OutputStreams>stdout</r:OutputStreams>`,
		`<creationXml xmlns="http://schemas.microsoft.com/powershell">abc</creationXml>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("createPSRPShellBody() missing %q in %s", want, body)
		}
	}
}

func TestReceiveBodyWithoutCommandID(t *testing.T) {
	body := receiveBody("", []string{"stdout"})
	if strings.Contains(body, "CommandId=") {
		t.Fatalf("receiveBody() = %s, want no CommandId", body)
	}
}

func TestSendBodyWithoutCommandID(t *testing.T) {
	body := sendBody("pr", "", []byte("hello"), true)
	for _, want := range []string{
		`Name="pr"`,
		`End="true"`,
		`aGVsbG8=`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("sendBody() missing %q in %s", want, body)
		}
	}
	if strings.Contains(body, "CommandId=") {
		t.Fatalf("sendBody() = %s, want no CommandId", body)
	}
}

func TestExecutePSRPCommandBody(t *testing.T) {
	body := executePSRPCommandBody(psrp.Fragment{
		ObjectID:   1,
		FragmentID: 0,
		Start:      true,
		End:        true,
		Blob: psrp.EncodeMessage(psrp.Message{
			Destination:    psrp.DestinationServer,
			Type:           psrp.MessageCreatePipeline,
			RunspacePoolID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			PipelineID:     uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			Data:           []byte("payload"),
		}),
	})

	if !strings.Contains(body, "<r:Command>") {
		t.Fatalf("executePSRPCommandBody() missing command element: %s", body)
	}
}

func TestBuildPSRPCommandPayloads(t *testing.T) {
	message := psrp.Message{
		Destination:    psrp.DestinationServer,
		Type:           psrp.MessageCreatePipeline,
		RunspacePoolID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		PipelineID:     uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Data:           []byte(strings.Repeat("x", 64)),
	}

	fragments := fragmentPipelineMessage(message, 8)
	first, continuations, err := buildPSRPCommandPayloads(fragments)
	if err != nil {
		t.Fatalf("buildPSRPCommandPayloads() error = %v", err)
	}
	if !first.Start {
		t.Fatal("first fragment should be start fragment")
	}
	if len(continuations) == 0 {
		t.Fatal("continuations should not be empty")
	}
	for _, payload := range continuations {
		fragment, err := psrp.DecodeFragment(payload)
		if err != nil {
			t.Fatalf("DecodeFragment() error = %v", err)
		}
		if fragment.Start {
			t.Fatal("continuation fragment should not be start fragment")
		}
	}
}
