//go:build !windows

package lsmuxsession

import "testing"

func TestParseAttachBindingByteDefaults(t *testing.T) {
	got, err := parseAttachBindingByte("", 0x01)
	if err != nil {
		t.Fatalf("parseAttachBindingByte returned error: %v", err)
	}
	if got != 0x01 {
		t.Fatalf("parseAttachBindingByte default = %d, want %d", got, 0x01)
	}
}

func TestParseAttachBindingByteCtrlSpec(t *testing.T) {
	got, err := parseAttachBindingByte("Ctrl+B", 0x01)
	if err != nil {
		t.Fatalf("parseAttachBindingByte returned error: %v", err)
	}
	if got != 0x02 {
		t.Fatalf("parseAttachBindingByte Ctrl+B = %d, want %d", got, 0x02)
	}
}

func TestParseAttachBindingByteRuneSpec(t *testing.T) {
	got, err := parseAttachBindingByte("x", 'd')
	if err != nil {
		t.Fatalf("parseAttachBindingByte returned error: %v", err)
	}
	if got != 'x' {
		t.Fatalf("parseAttachBindingByte x = %d, want %d", got, 'x')
	}
}
