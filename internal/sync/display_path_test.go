package sync

import (
	"path/filepath"
	"testing"
)

func TestDisplayLocalPathPreservesRelativeRoot(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp", "repo", "src")
	current := filepath.Join(root, "nested", "file.txt")

	got, ok := displayLocalPath(root, "."+string(filepath.Separator)+"src", current)
	if !ok {
		t.Fatal("displayLocalPath() = not matched, want matched")
	}

	want := "." + string(filepath.Separator) + filepath.Join("src", "nested", "file.txt")
	if got != want {
		t.Fatalf("displayLocalPath() = %q, want %q", got, want)
	}
}

func TestDestinationDisplayResolverAppendsServerForRemoteToLocal(t *testing.T) {
	resolver := destinationDisplayResolver(
		filepath.Join(string(filepath.Separator), "tmp", "repo", "backup"),
		"."+string(filepath.Separator)+"backup",
		true,
		"web01",
	)

	got := resolver(filepath.Join(string(filepath.Separator), "tmp", "repo", "backup", "web01", "app", "config.yml"))
	want := "." + string(filepath.Separator) + filepath.Join("backup", "web01", "app", "config.yml")
	if got != want {
		t.Fatalf("resolver() = %q, want %q", got, want)
	}
}
