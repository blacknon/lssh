package ssmsession

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/blacknon/lssh/internal/connectorruntime"
	"github.com/google/uuid"
)

type discardConn struct{}

func (discardConn) Read([]byte) (int, error)         { return 0, nil }
func (discardConn) Write(p []byte) (int, error)      { return len(p), nil }
func (discardConn) Close() error                     { return nil }
func (discardConn) LocalAddr() net.Addr              { return nil }
func (discardConn) RemoteAddr() net.Addr             { return nil }
func (discardConn) SetDeadline(time.Time) error      { return nil }
func (discardConn) SetReadDeadline(time.Time) error  { return nil }
func (discardConn) SetWriteDeadline(time.Time) error { return nil }

func TestEncodeDecodeClientMessage(t *testing.T) {
	original := clientMessage{
		MessageType:    messageTypeInputStream,
		SchemaVersion:  1,
		CreatedDate:    123456789,
		SequenceNumber: 42,
		Flags:          7,
		MessageID:      uuid.New(),
		PayloadType:    payloadTypeOutput,
		Payload:        []byte("hello"),
	}

	raw := encodeClientMessage(original)
	decoded, err := decodeClientMessage(raw)
	if err != nil {
		t.Fatalf("decodeClientMessage() error = %v", err)
	}
	if decoded.MessageType != original.MessageType {
		t.Fatalf("MessageType = %q, want %q", decoded.MessageType, original.MessageType)
	}
	if decoded.SequenceNumber != original.SequenceNumber {
		t.Fatalf("SequenceNumber = %d, want %d", decoded.SequenceNumber, original.SequenceNumber)
	}
	if decoded.PayloadType != original.PayloadType {
		t.Fatalf("PayloadType = %d, want %d", decoded.PayloadType, original.PayloadType)
	}
	if string(decoded.Payload) != string(original.Payload) {
		t.Fatalf("Payload = %q, want %q", decoded.Payload, original.Payload)
	}
}

func TestIsPresignedURL(t *testing.T) {
	if !isPresignedURL("wss://example.amazonaws.com/v1/data-channel/test?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=abc") {
		t.Fatal("isPresignedURL() = false, want true")
	}
	if isPresignedURL("wss://example.amazonaws.com/v1/data-channel/test") {
		t.Fatal("isPresignedURL() = true, want false")
	}
}

func TestHandleOutputMessageMarksReadyWithoutHandshakeComplete(t *testing.T) {
	session := New(Config{})
	session.conn = &wsConn{conn: discardConn{}}
	msg := clientMessage{
		MessageType:    messageTypeOutputStream,
		MessageID:      uuid.New(),
		SequenceNumber: 0,
		PayloadType:    payloadTypeOutput,
		Payload:        []byte("sh-5.2$ "),
	}

	var stdout bytes.Buffer
	if err := session.handleOutputMessage(msg, &stdout, nil); err != nil {
		t.Fatalf("handleOutputMessage() error = %v", err)
	}

	select {
	case <-session.ready:
	default:
		t.Fatal("session.ready was not closed by first output message")
	}
	select {
	case <-session.startupTrigger:
	default:
		t.Fatal("session.startupTrigger was not closed by first output message")
	}

	if stdout.String() != "sh-5.2$ " {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "sh-5.2$ ")
	}
}

func TestBuildStreamCommandLineWrapsCommand(t *testing.T) {
	command := buildStreamCommandLine("cat > /tmp/test")
	if !strings.Contains(command, "sh -c") {
		t.Fatalf("buildStreamCommandLine() = %q, want sh -c wrapper", command)
	}
	if !strings.Contains(command, streamExitMarkerPrefix) {
		t.Fatalf("buildStreamCommandLine() = %q, want exit marker", command)
	}
}

func TestStreamExitStatusWriterParsesExitCode(t *testing.T) {
	var stderr bytes.Buffer
	writer := newStreamExitStatusWriter(&stderr)

	if _, err := writer.Write([]byte("warn\n")); err != nil {
		t.Fatalf("Write(stderr) error = %v", err)
	}
	if _, err := writer.Write([]byte(streamExitMarkerPrefix + "7\n")); err != nil {
		t.Fatalf("Write(marker) error = %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	code, ok := writer.ExitCode()
	if !ok || code != 7 {
		t.Fatalf("ExitCode() = (%d, %v), want (7, true)", code, ok)
	}
	if stderr.String() != "warn\n" {
		t.Fatalf("stderr = %q, want %q", stderr.String(), "warn\n")
	}
}

func TestSendStartupCommandSplitsLongInput(t *testing.T) {
	session := New(Config{})
	session.conn = &wsConn{conn: discardConn{}}

	longCommand := strings.Repeat("x", 1200)
	if err := session.sendStartupCommand(longCommand); err != nil {
		t.Fatalf("sendStartupCommand() error = %v", err)
	}

	if session.sequence != 4 {
		t.Fatalf("sequence = %d, want 4 (3 chunks + enter)", session.sequence)
	}
}

func TestBuildStartupCommandAddsMarker(t *testing.T) {
	got := BuildStartupCommand("stty size", StartupEchoMarker())
	if !strings.Contains(got, "printf '%s\\n' __LSSH_STARTUP__") {
		t.Fatalf("BuildStartupCommand() = %q, want startup marker print", got)
	}
	if !strings.Contains(got, "sh -c 'stty size'") {
		t.Fatalf("BuildStartupCommand() = %q, want wrapped command", got)
	}
}

func TestNormalizeInteractiveInputPayload(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "single newline", in: "\n", want: "\r"},
		{name: "command line", in: "pwd\n", want: "pwd\r"},
		{name: "collapse crlf", in: "pwd\r\n", want: "pwd\r"},
		{name: "keep cr", in: "pwd\r", want: "pwd\r"},
		{name: "multiple lines", in: "a\nb\n", want: "a\rb\r"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(normalizeInteractiveInputPayload([]byte(tt.in), false))
			if got != tt.want {
				t.Fatalf("normalizeInteractiveInputPayload(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeInteractiveInputPayloadExpandCRLF(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "single cr", in: "\r", want: "\r\n"},
		{name: "single newline", in: "\n", want: "\r\n"},
		{name: "command line", in: "pwd\r", want: "pwd\r\n"},
		{name: "keep crlf", in: "pwd\r\n", want: "pwd\r\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(normalizeInteractiveInputPayload([]byte(tt.in), true))
			if got != tt.want {
				t.Fatalf("normalizeInteractiveInputPayload(%q, true) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFilterStartupOutputSuppressesUntilMarker(t *testing.T) {
	session := New(Config{StartupMarker: "__READY__"})
	session.startupSuppress = true
	session.startupSince = time.Now()

	if got := session.filterStartupOutput([]byte("sh-5.2$ long startup")); got != nil {
		t.Fatalf("filterStartupOutput(before marker) = %q, want nil", string(got))
	}

	got := session.filterStartupOutput([]byte("__READY__\r\nprompt$ "))
	if string(got) != "prompt$ " {
		t.Fatalf("filterStartupOutput(after marker) = %q, want %q", string(got), "prompt$ ")
	}
	if session.startupSuppress {
		t.Fatal("startupSuppress = true, want false")
	}
}

func TestFilterStartupOutputFallsBackAfterTimeout(t *testing.T) {
	session := New(Config{StartupMarker: "__READY__"})
	session.startupMu.Lock()
	session.startupSuppress = true
	session.startupSince = time.Now().Add(-startupMarkerWaitLimit - time.Second)
	session.startupMu.Unlock()

	got := session.filterStartupOutput([]byte("plain prompt$ "))
	if string(got) != "plain prompt$ " {
		t.Fatalf("filterStartupOutput(timeout) = %q, want %q", string(got), "plain prompt$ ")
	}
	if session.startupSuppress {
		t.Fatal("startupSuppress = true, want false after timeout fallback")
	}
}

func TestStartStartupSuppressTimesOut(t *testing.T) {
	session := New(Config{StartupMarker: "__READY__"})
	session.startStartupSuppress()

	time.Sleep(startupMarkerWaitLimit + 200*time.Millisecond)

	session.startupMu.Lock()
	defer session.startupMu.Unlock()
	if session.startupSuppress {
		t.Fatal("startupSuppress = true, want false after watchdog timeout")
	}
}

func TestResizeDefersUntilReady(t *testing.T) {
	session := New(Config{})
	if err := session.Resize(context.Background(), connectorruntime.TerminalSize{Cols: 128, Rows: 27}); err != nil {
		t.Fatalf("Resize() error = %v", err)
	}

	session.sizeMu.Lock()
	defer session.sizeMu.Unlock()
	if !session.hasPendingSize {
		t.Fatal("hasPendingSize = false, want true")
	}
	if session.pendingResize.Cols != 128 || session.pendingResize.Rows != 27 {
		t.Fatalf("pendingResize = %+v, want 128x27", session.pendingResize)
	}
}

func TestHandleHandshakeRequestAcceptsPortSession(t *testing.T) {
	session := New(Config{})
	session.conn = &wsConn{conn: discardConn{}}

	requestBody, err := json.Marshal(handshakeRequestPayload{
		RequestedClientActions: []requestedClientAction{
			{
				ActionType: "SessionType",
				ActionParameters: mustMarshalJSON(t, sessionTypeRequest{
					SessionType: sessionTypePort,
				}),
			},
		},
	})
	if err != nil {
		t.Fatalf("Marshal(handshakeRequestPayload) error = %v", err)
	}

	if err := session.handleHandshakeRequest(requestBody); err != nil {
		t.Fatalf("handleHandshakeRequest() error = %v", err)
	}
	if session.sessionType != sessionTypePort {
		t.Fatalf("session.sessionType = %q, want %q", session.sessionType, sessionTypePort)
	}
}

func TestHandleFlagMessageConnectError(t *testing.T) {
	session := New(Config{})
	payload := make([]byte, 4)
	payload[3] = byte(portFlagConnectError)

	err := session.handleFlagMessage(payload)
	if err == nil {
		t.Fatal("handleFlagMessage() error = nil, want non-nil")
	}
}

func TestStartPortSessionRelayDoesNotWaitForReady(t *testing.T) {
	session := New(Config{})

	conn := startPortSessionRelay(context.Background(), session)
	if conn == nil {
		t.Fatal("startPortSessionRelay() = nil, want non-nil")
	}
	defer conn.Close()

	select {
	case <-session.ready:
		t.Fatal("session.ready was closed unexpectedly")
	default:
	}
}

func mustMarshalJSON(t *testing.T, value interface{}) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return raw
}
