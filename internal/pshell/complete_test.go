package pshell

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/blacknon/go-sshlib"
	"github.com/c-bata/go-prompt"
)

func TestLocalCommandSuggests(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits differ on windows")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "run-me"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile executable failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("nope"), 0o644); err != nil {
		t.Fatalf("WriteFile plain failed: %v", err)
	}

	t.Setenv("PATH", dir)

	suggests := localCommandSuggests()
	if len(suggests) != 1 || suggests[0].Text != "run-me" {
		t.Fatalf("localCommandSuggests() = %#v, want run-me only", suggests)
	}
}

func TestLocalPathSuggests(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("Mkdir subdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile alpha failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "beta.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("WriteFile beta failed: %v", err)
	}

	suggests := localPathSuggests(filepath.Join(dir, "a"))
	if len(suggests) != 1 || suggests[0].Text != "alpha.txt" {
		t.Fatalf("localPathSuggests() = %#v, want %q only", suggests, "alpha.txt")
	}

	dirSuggests := localPathSuggests(filepath.Join(dir, "s"))
	if len(dirSuggests) != 1 || dirSuggests[0].Text != "subdir" {
		t.Fatalf("localPathSuggests() dir = %#v, want %q only", dirSuggests, "subdir")
	}
}

func TestGetPathCompleteForConnectsRemote(t *testing.T) {
	orig := runCompleteCommandFn
	t.Cleanup(func() {
		runCompleteCommandFn = orig
	})

	runCompleteCommandFn = func(c *sConnect, command string) (*bytes.Buffer, error) {
		if command != remotePathCompleteCommand("/ho") {
			t.Fatalf("unexpected command: %q", command)
		}

		buf := new(bytes.Buffer)
		buf.WriteString("/home/\n")
		buf.WriteString("/hoge.txt\n")
		return buf, nil
	}

	s := &shell{}
	connects := []*sConnect{
		{
			Name:      "web-1",
			Connected: true,
			Connect:   &sshlib.Connect{},
		},
	}

	got := s.GetPathCompleteForConnects(connects, true, "/ho")
	if len(got) != 2 {
		t.Fatalf("GetPathCompleteForConnects() count = %d, want 2 (%#v)", len(got), got)
	}
	gotMap := map[string]bool{}
	for _, suggest := range got {
		gotMap[suggest.Text] = true
	}
	if !gotMap["home"] || !gotMap["hoge.txt"] {
		t.Fatalf("GetPathCompleteForConnects() = %#v, want home and hoge.txt", got)
	}
}

func TestCompleterRemotePathAfterCommand(t *testing.T) {
	orig := runCompleteCommandFn
	t.Cleanup(func() {
		runCompleteCommandFn = orig
	})

	runCompleteCommandFn = func(c *sConnect, command string) (*bytes.Buffer, error) {
		if command != remotePathCompleteCommand("/ho") {
			t.Fatalf("unexpected command: %q", command)
		}

		buf := new(bytes.Buffer)
		buf.WriteString("/home/\n")
		buf.WriteString("/hoge.txt\n")
		return buf, nil
	}

	s := &shell{
		Connects: []*sConnect{
			{
				Name:      "web-1",
				Connected: true,
				Connect:   &sshlib.Connect{},
			},
		},
	}

	buf := prompt.NewBuffer()
	buf.InsertText("ls /ho", false, true)

	got := s.Completer(*buf.Document())
	if len(got) != 2 {
		t.Fatalf("Completer() count = %d, want 2 (%#v)", len(got), got)
	}
	gotMap := map[string]bool{}
	for _, suggest := range got {
		gotMap[suggest.Text] = true
	}
	if !gotMap["home"] || !gotMap["hoge.txt"] {
		t.Fatalf("Completer() = %#v, want home and hoge.txt", got)
	}
}

func TestPathCompletionHelpers(t *testing.T) {
	tests := []struct {
		name      string
		word      string
		candidate string
		wantWord  string
		wantText  string
	}{
		{name: "absolute path", word: "/etc/Net", candidate: "/etc/NetworkManager/", wantWord: "Net", wantText: "NetworkManager"},
		{name: "double trailing slash", word: "/bin/zoo", candidate: "/bin/zoo-3//", wantWord: "zoo", wantText: "zoo-3"},
		{name: "relative path", word: "./alp", candidate: "./alpha.txt", wantWord: "alp", wantText: "alpha.txt"},
		{name: "simple name", word: "alp", candidate: "alpha.txt", wantWord: "alp", wantText: "alpha.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathCompletionFilterWord(tt.word); got != tt.wantWord {
				t.Fatalf("pathCompletionFilterWord(%q) = %q, want %q", tt.word, got, tt.wantWord)
			}
			if got := pathCompletionText(tt.candidate); got != tt.wantText {
				t.Fatalf("pathCompletionText(%q) = %q, want %q", tt.candidate, got, tt.wantText)
			}
		})
	}
}

func TestRemotePathCompleteCommandNormalizesDirectorySuffix(t *testing.T) {
	got := remotePathCompleteCommand("/bin/zo")
	if !strings.Contains(got, "p=${p%/}") {
		t.Fatalf("remotePathCompleteCommand() = %q, want directory suffix normalization", got)
	}
}
