package ssmsession

import (
	"bytes"
	"net"
	"strings"
	"testing"
	"time"

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

	if stdout.String() != "sh-5.2$ " {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "sh-5.2$ ")
	}
}

func TestBuildStreamCommandLineWrapsCommand(t *testing.T) {
	command := buildStreamCommandLine("cat > /tmp/test")
	if !strings.Contains(command, "sh -lc") {
		t.Fatalf("buildStreamCommandLine() = %q, want sh -lc wrapper", command)
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

func TestFilterStartupOutputSuppressesUntilMarker(t *testing.T) {
	session := New(Config{StartupMarker: "__READY__"})
	session.startupSuppress = true

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
