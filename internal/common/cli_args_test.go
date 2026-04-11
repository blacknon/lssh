package common

import (
	"reflect"
	"testing"
)

func TestNormalizeTrailingBoolFlags(t *testing.T) {
	flags := map[string]bool{
		"--foreground": true,
		"--rw":         true,
	}
	valueFlags := map[string]bool{
		"-H":     true,
		"--host": true,
		"-F":     true,
		"--file": true,
	}

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "moves trailing foreground",
			args: []string{"lsshfs", "/tmp/src", "/tmp/mnt", "--foreground"},
			want: []string{"lsshfs", "--foreground", "/tmp/src", "/tmp/mnt"},
		},
		{
			name: "moves multiple trailing flags",
			args: []string{"lsshfs", "-H", "web01", "/tmp/src", "/tmp/mnt", "--rw", "--foreground"},
			want: []string{"lsshfs", "-H", "web01", "--rw", "--foreground", "/tmp/src", "/tmp/mnt"},
		},
		{
			name: "keeps regular arguments",
			args: []string{"lsshfs", "--foreground", "/tmp/src", "/tmp/mnt"},
			want: []string{"lsshfs", "--foreground", "/tmp/src", "/tmp/mnt"},
		},
	}

	for _, tt := range tests {
		got := NormalizeTrailingBoolFlags(tt.args, flags, valueFlags)
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%s: NormalizeTrailingBoolFlags() = %#v, want %#v", tt.name, got, tt.want)
		}
	}
}
