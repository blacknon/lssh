package lspipe

import (
	"reflect"
	"testing"
)

func TestResolveExecHosts(t *testing.T) {
	t.Parallel()

	sessionHosts := []string{"web01", "web02", "db01"}

	tests := []struct {
		name      string
		selectors []string
		want      []string
		wantErr   bool
	}{
		{
			name:      "no selectors",
			selectors: nil,
			want:      nil,
		},
		{
			name:      "host names",
			selectors: []string{"web02"},
			want:      []string{"web02"},
		},
		{
			name:      "one based indexes",
			selectors: []string{"1", "3"},
			want:      []string{"web01", "db01"},
		},
		{
			name:      "mixed names and indexes",
			selectors: []string{"2", "db01"},
			want:      []string{"web02", "db01"},
		},
		{
			name:      "deduplicate",
			selectors: []string{"1", "web01"},
			want:      []string{"web01"},
		},
		{
			name:      "out of range",
			selectors: []string{"4"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveExecHosts(sessionHosts, tt.selectors)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ResolveExecHosts() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveExecHosts() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ResolveExecHosts() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
