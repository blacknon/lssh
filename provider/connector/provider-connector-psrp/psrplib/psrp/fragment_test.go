package psrp

import (
	"bytes"
	"strings"
	"testing"
)

func TestEncodeDecodeFragmentRoundTrip(t *testing.T) {
	original := Fragment{
		ObjectID:   10,
		FragmentID: 2,
		Start:      false,
		End:        true,
		Blob:       []byte("hello"),
	}

	raw := EncodeFragment(original)
	decoded, err := DecodeFragment(raw)
	if err != nil {
		t.Fatalf("DecodeFragment() error = %v", err)
	}

	if decoded.ObjectID != original.ObjectID {
		t.Fatalf("ObjectID = %d, want %d", decoded.ObjectID, original.ObjectID)
	}
	if decoded.FragmentID != original.FragmentID {
		t.Fatalf("FragmentID = %d, want %d", decoded.FragmentID, original.FragmentID)
	}
	if decoded.Start != original.Start || decoded.End != original.End {
		t.Fatalf("Start/End = %v/%v, want %v/%v", decoded.Start, decoded.End, original.Start, original.End)
	}
	if !bytes.Equal(decoded.Blob, original.Blob) {
		t.Fatalf("Blob = %q, want %q", decoded.Blob, original.Blob)
	}
}

func TestFragmentMessage(t *testing.T) {
	fragments := FragmentMessage(7, []byte("abcdef"), 2)
	if len(fragments) != 3 {
		t.Fatalf("len(fragments) = %d, want 3", len(fragments))
	}
	if !fragments[0].Start || fragments[0].End {
		t.Fatalf("first fragment Start/End = %v/%v, want true/false", fragments[0].Start, fragments[0].End)
	}
	if fragments[2].Start || !fragments[2].End {
		t.Fatalf("last fragment Start/End = %v/%v, want false/true", fragments[2].Start, fragments[2].End)
	}
}

func TestBuildCreationXML(t *testing.T) {
	raw, err := BuildCreationXML([]Fragment{{
		ObjectID:   1,
		FragmentID: 0,
		Start:      true,
		End:        true,
		Blob:       []byte("payload"),
	}})
	if err != nil {
		t.Fatalf("BuildCreationXML() error = %v", err)
	}
	if !strings.Contains(string(raw), "<creationXml") {
		t.Fatalf("creation xml missing root element: %s", raw)
	}
}

func TestDecodeFragmentsAndAssembleFragments(t *testing.T) {
	message := []byte("hello world")
	fragments := FragmentMessage(3, message, 4)

	var raw bytes.Buffer
	for _, fragment := range fragments {
		raw.Write(EncodeFragment(fragment))
	}

	decoded, err := DecodeFragments(raw.Bytes())
	if err != nil {
		t.Fatalf("DecodeFragments() error = %v", err)
	}

	assembled, err := AssembleFragments(decoded)
	if err != nil {
		t.Fatalf("AssembleFragments() error = %v", err)
	}

	if !bytes.Equal(assembled, message) {
		t.Fatalf("assembled message = %q, want %q", assembled, message)
	}
}

func TestDecodeMessages(t *testing.T) {
	message1 := EncodeMessage(Message{
		Destination: DestinationServer,
		Type:        MessageSessionCapability,
		Data:        []byte("one"),
	})
	message2 := EncodeMessage(Message{
		Destination: DestinationServer,
		Type:        MessageInitRunspacePool,
		Data:        []byte("two"),
	})

	var raw bytes.Buffer
	for _, fragment := range append(FragmentMessage(1, message1, 3), FragmentMessage(2, message2, 3)...) {
		raw.Write(EncodeFragment(fragment))
	}

	messages, err := DecodeMessages(raw.Bytes())
	if err != nil {
		t.Fatalf("DecodeMessages() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(messages))
	}
	if string(messages[0].Data) != "one" {
		t.Fatalf("messages[0].Data = %q, want one", messages[0].Data)
	}
	if string(messages[1].Data) != "two" {
		t.Fatalf("messages[1].Data = %q, want two", messages[1].Data)
	}
}
