package telnetlib

import (
	"bytes"
	"testing"
)

func TestEscapedWriterWrite(t *testing.T) {
	var dst bytes.Buffer
	writer := &escapedWriter{w: &dst}

	if _, err := writer.Write([]byte("hello\nworld\r\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if got, want := dst.String(), "hello\r\nworld\r\n"; got != want {
		t.Fatalf("escaped output = %q, want %q", got, want)
	}
}

func TestFilterTelnetCommands(t *testing.T) {
	src := []byte{'o', 'k', iac, will, 1, '!', iac, iac}
	got := string(filterTelnetCommands(src))
	if got != "ok!\xff" {
		t.Fatalf("filterTelnetCommands() = %q, want %q", got, "ok!\xff")
	}
}
