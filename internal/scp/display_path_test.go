package scp

import (
	"path/filepath"
	"testing"
)

func TestDisplayLocalPathPreservesRelativeRoot(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp", "repo", "src")
	current := filepath.Join(root, "nested", "file.txt")

	got := displayLocalPath(root, "."+string(filepath.Separator)+"src", current)
	want := "." + string(filepath.Separator) + filepath.Join("src", "nested", "file.txt")
	if got != want {
		t.Fatalf("displayLocalPath() = %q, want %q", got, want)
	}
}

func TestDisplayLocalPathFallsBackToDisplayRootForRootFile(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp", "repo", "file.txt")

	got := displayLocalPath(root, "."+string(filepath.Separator)+"file.txt", root)
	want := "." + string(filepath.Separator) + "file.txt"
	if got != want {
		t.Fatalf("displayLocalPath() = %q, want %q", got, want)
	}
}
