package check

import (
	"strings"
	"testing"
)

func TestExistServerRequiresAllHostsToExist(t *testing.T) {
	t.Parallel()

	if !ExistServer([]string{"web1", "web2"}, []string{"web1", "web2", "db1"}) {
		t.Fatalf("expected all existing hosts to pass")
	}

	if ExistServer([]string{"web1", "missing"}, []string{"web1", "web2", "db1"}) {
		t.Fatalf("expected missing host to fail validation")
	}
}

func TestParseScpPathE(t *testing.T) {
	t.Parallel()

	tests := []struct {
		arg        string
		wantRemote bool
		wantPath   string
		wantErr    string
	}{
		{arg: "/tmp/a.txt", wantPath: "/tmp/a.txt"},
		{arg: "remote:/tmp/a.txt", wantRemote: true, wantPath: "/tmp/a.txt"},
		{arg: "local:/tmp/a.txt", wantPath: "/tmp/a.txt"},
		{arg: "bad:/tmp/a.txt", wantErr: "incorrect"},
	}

	for _, tt := range tests {
		gotRemote, gotPath, err := ParseScpPathE(tt.arg)
		if tt.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ParseScpPathE(%q) error = %v", tt.arg, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseScpPathE(%q) error = %v", tt.arg, err)
		}
		if gotRemote != tt.wantRemote || gotPath != tt.wantPath {
			t.Fatalf("ParseScpPathE(%q) = (%t, %q)", tt.arg, gotRemote, gotPath)
		}
	}
}

func TestValidateCopyTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		remote  bool
		local   bool
		to      bool
		hosts   int
		wantErr string
	}{
		{name: "mixed", remote: true, local: true, wantErr: "Can not set LOCAL and REMOTE"},
		{name: "local to local", local: true, wantErr: "It does not correspond LOCAL to LOCAL"},
		{name: "remote remote with hosts", remote: true, to: true, hosts: 1, wantErr: "does not correspond to host option"},
		{name: "valid", remote: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCopyTypes(tt.remote, tt.local, tt.to, tt.hosts)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateCopyTypes() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateCopyTypes() error = %v", err)
			}
		})
	}
}
