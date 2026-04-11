package lspipe

import "testing"

func TestShouldSuppressSttyNoise(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"stty: standard input: Inappropriate ioctl for device\n", true},
		{"stty: 標準入力: デバイスに対する不適切なioctlです\n", true},
		{"stty: 'stdin': Inappropriate ioctl for device\n", true},
		{"stty: invalid argument\n", false},
		{"warning: something else\n", false},
	}

	for _, tt := range cases {
		if got := shouldSuppressSttyNoise(tt.line); got != tt.want {
			t.Fatalf("shouldSuppressSttyNoise(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestFilterSttyNoiseBytes(t *testing.T) {
	in := []byte("stty: standard input: Inappropriate ioctl for device\nhello\n")
	got := string(filterSttyNoiseBytes(in))
	if got != "hello\n" {
		t.Fatalf("filterSttyNoiseBytes() = %q, want %q", got, "hello\n")
	}
}
