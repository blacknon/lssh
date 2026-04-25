package psrp

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
)

func TestEncodeDecodeMessageRoundTrip(t *testing.T) {
	original := Message{
		Destination:    DestinationServer,
		Type:           MessageCreatePipeline,
		RunspacePoolID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		PipelineID:     uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Data:           []byte("payload"),
	}

	raw := EncodeMessage(original)
	decoded, err := DecodeMessage(raw)
	if err != nil {
		t.Fatalf("DecodeMessage() error = %v", err)
	}

	if decoded.Destination != original.Destination {
		t.Fatalf("Destination = %v, want %v", decoded.Destination, original.Destination)
	}
	if decoded.Type != original.Type {
		t.Fatalf("Type = %v, want %v", decoded.Type, original.Type)
	}
	if decoded.RunspacePoolID != original.RunspacePoolID {
		t.Fatalf("RunspacePoolID = %v, want %v", decoded.RunspacePoolID, original.RunspacePoolID)
	}
	if decoded.PipelineID != original.PipelineID {
		t.Fatalf("PipelineID = %v, want %v", decoded.PipelineID, original.PipelineID)
	}
	if !bytes.Equal(decoded.Data, original.Data) {
		t.Fatalf("Data = %q, want %q", decoded.Data, original.Data)
	}
}

func TestDecodeMessageRejectsShortBuffer(t *testing.T) {
	if _, err := DecodeMessage([]byte("short")); err == nil {
		t.Fatal("DecodeMessage(short) error = nil, want error")
	}
}

func TestEncodeMessageUsesLittleEndianHeaderAndGUIDPacketFormat(t *testing.T) {
	message := Message{
		Destination:    DestinationServer,
		Type:           MessageCreatePipeline,
		RunspacePoolID: uuid.MustParse("00112233-4455-6677-8899-aabbccddeeff"),
		PipelineID:     uuid.MustParse("fedcba98-7654-3210-0123-456789abcdef"),
		Data:           []byte("x"),
	}

	raw := EncodeMessage(message)
	if got := hex.EncodeToString(raw[:40]); got != "020000000610020033221100554477668899aabbccddeeff98badcfe547610320123456789abcdef" {
		t.Fatalf("EncodeMessage() header = %s", got)
	}
}
