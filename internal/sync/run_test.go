package sync

import "testing"

func TestHasPathPrefix(t *testing.T) {
	t.Parallel()

	if !hasPathPrefix("/dest/app/file.txt", "/dest/app", func(path string) string { return path }, "/") {
		t.Fatalf("expected child path to be inside scope")
	}
	if hasPathPrefix("/dest/app2/file.txt", "/dest/app", func(path string) string { return path }, "/") {
		t.Fatalf("expected sibling path to be outside scope")
	}
}
