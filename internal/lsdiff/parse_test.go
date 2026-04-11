package lsdiff

import "testing"

func TestParseTargetSpec(t *testing.T) {
	tests := []struct {
		value string
		host  string
		path  string
		ok    bool
	}{
		{value: "@host1:/tmp/a", host: "host1", path: "/tmp/a", ok: true},
		{value: "host1:/tmp/a", host: "host1", path: "/tmp/a", ok: true},
		{value: "/tmp/a", host: "", path: "/tmp/a", ok: true},
		{value: "@host1", ok: false},
	}

	for _, tt := range tests {
		target, err := ParseTargetSpec(tt.value)
		if tt.ok && err != nil {
			t.Fatalf("ParseTargetSpec(%q) error = %v", tt.value, err)
		}
		if !tt.ok && err == nil {
			t.Fatalf("ParseTargetSpec(%q) error = nil", tt.value)
		}
		if tt.ok && (target.Host != tt.host || target.RemotePath != tt.path) {
			t.Fatalf("ParseTargetSpec(%q) = (%q,%q), want (%q,%q)", tt.value, target.Host, target.RemotePath, tt.host, tt.path)
		}
	}
}
