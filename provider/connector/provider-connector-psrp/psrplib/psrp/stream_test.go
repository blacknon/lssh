package psrp

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
)

func TestMessageStreamDecoderPush(t *testing.T) {
	message := EncodeMessage(Message{
		Destination:    DestinationClient,
		Type:           MessagePipelineOutput,
		RunspacePoolID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		PipelineID:     uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Data:           []byte("hello"),
	})
	fragments := FragmentMessage(1, message, 3)

	var raw bytes.Buffer
	for _, fragment := range fragments {
		raw.Write(EncodeFragment(fragment))
	}

	decoder := &MessageStreamDecoder{}
	first, err := decoder.Push(raw.Bytes()[:10])
	if err != nil {
		t.Fatalf("Push(first) error = %v", err)
	}
	if len(first) != 0 {
		t.Fatalf("len(first) = %d, want 0", len(first))
	}

	second, err := decoder.Push(raw.Bytes()[10:])
	if err != nil {
		t.Fatalf("Push(second) error = %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("len(second) = %d, want 1", len(second))
	}
	if string(second[0].Data) != "hello" {
		t.Fatalf("second[0].Data = %q, want hello", second[0].Data)
	}
	if err := decoder.Finalize(); err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
}
