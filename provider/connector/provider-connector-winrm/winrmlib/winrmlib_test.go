package winrmlib

import "testing"

func TestParseBoolDefault(t *testing.T) {
	if !parseBoolDefault("true", false) {
		t.Fatal("parseBoolDefault(true) = false, want true")
	}
	if parseBoolDefault("invalid", true) != true {
		t.Fatal("parseBoolDefault(invalid, true) != true")
	}
}

func TestParseIntDefault(t *testing.T) {
	if got := parseIntDefault("5986", 0); got != 5986 {
		t.Fatalf("parseIntDefault() = %d, want 5986", got)
	}
	if got := parseIntDefault("invalid", 60); got != 60 {
		t.Fatalf("parseIntDefault(invalid) = %d, want 60", got)
	}
}
