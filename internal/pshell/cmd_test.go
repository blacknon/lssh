package pshell

import (
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
)

func TestNormalizeLocalCommand(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "single plus", in: "+cat", want: "cat"},
		{name: "double plus", in: "++cat", want: "cat"},
		{name: "no plus", in: "cat", want: "cat"},
		{name: "only single plus", in: "+", want: ""},
		{name: "only double plus", in: "++", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeLocalCommand(tt.in)
			if got != tt.want {
				t.Fatalf("normalizeLocalCommand(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCheckLocalCommand(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "single plus command", in: "+cat", want: true},
		{name: "double plus command", in: "++cat", want: true},
		{name: "single plus only", in: "+", want: true},
		{name: "double plus only", in: "++", want: true},
		{name: "plain command", in: "cat", want: false},
		{name: "empty", in: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkLocalCommand(tt.in)
			if got != tt.want {
				t.Fatalf("checkLocalCommand(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestCheckBuildInCommand(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "sync", in: "%sync", want: true},
		{name: "get", in: "%get", want: true},
		{name: "status", in: "%status", want: true},
		{name: "reconnect", in: "%reconnect", want: true},
		{name: "plain remote", in: "hostname", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkBuildInCommand(tt.in)
			if got != tt.want {
				t.Fatalf("checkBuildInCommand(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizePerHostPipeLine(t *testing.T) {
	pline := []pipeLine{
		{Args: []string{"hostname"}, Oprator: "|"},
		{Args: []string{"++cat"}, Oprator: "|"},
		{Args: []string{"cat"}},
	}

	got := normalizePerHostPipeLine(pline)
	if got[1].Args[0] != "+cat" {
		t.Fatalf("normalizePerHostPipeLine() middle command = %q, want %q", got[1].Args[0], "+cat")
	}

	if pline[1].Args[0] != "++cat" {
		t.Fatalf("normalizePerHostPipeLine() mutated original pipeline")
	}
}

func TestPipelineScopedConnects(t *testing.T) {
	s := &shell{
		currentConns: []*sConnect{
			{Name: "web01"},
			{Name: "web02"},
			{Name: "db01"},
		},
	}

	tests := []struct {
		name string
		in   []pipeLine
		want []string
	}{
		{
			name: "no targeted command uses all active connects",
			in: []pipeLine{
				{Args: []string{"hostname"}, Oprator: "|"},
				{Args: []string{"++cat"}},
			},
			want: []string{"web01", "web02", "db01"},
		},
		{
			name: "targeted command narrows connects",
			in: []pipeLine{
				{Args: []string{"@web01,db01:hostname"}, Oprator: "|"},
				{Args: []string{"++cat"}},
			},
			want: []string{"web01", "db01"},
		},
		{
			name: "multiple targeted commands intersect connects",
			in: []pipeLine{
				{Args: []string{"@web01,web02:hostname"}, Oprator: "|"},
				{Args: []string{"++cat"}, Oprator: "|"},
				{Args: []string{"@web02:cat"}},
			},
			want: []string{"web02"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.pipelineScopedConnects(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("pipelineScopedConnects() len = %d, want %d", len(got), len(tt.want))
			}

			for i, conn := range got {
				if conn.Name != tt.want[i] {
					t.Fatalf("pipelineScopedConnects()[%d] = %q, want %q", i, conn.Name, tt.want[i])
				}
			}
		})
	}
}

func TestExpandPipeLineAliases(t *testing.T) {
	s := &shell{
		Config: conf.ShellConfig{
			Alias: map[string]conf.ShellAliasConfig{
				"ll": {Command: "ls -lah"},
			},
		},
	}

	tests := []struct {
		name string
		in   []pipeLine
		want []string
	}{
		{
			name: "simple alias",
			in: []pipeLine{
				{Args: []string{"ll", "/tmp"}},
			},
			want: []string{"ls", "-lah", "/tmp"},
		},
		{
			name: "targeted alias",
			in: []pipeLine{
				{Args: []string{"@web01:ll"}},
			},
			want: []string{"@web01:ls", "-lah"},
		},
		{
			name: "builtin untouched",
			in: []pipeLine{
				{Args: []string{"%history"}},
			},
			want: []string{"%history"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.expandPipeLineAliases(tt.in)
			if len(got) != 1 {
				t.Fatalf("expandPipeLineAliases() len = %d, want 1", len(got))
			}
			if len(got[0].Args) != len(tt.want) {
				t.Fatalf("expandPipeLineAliases() args len = %d, want %d: %#v", len(got[0].Args), len(tt.want), got[0].Args)
			}
			for i := range tt.want {
				if got[0].Args[i] != tt.want[i] {
					t.Fatalf("expandPipeLineAliases()[%d] = %q, want %q", i, got[0].Args[i], tt.want[i])
				}
			}
		})
	}
}
