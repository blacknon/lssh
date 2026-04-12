//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package sshlib

import "testing"

func TestBillyPathFSCleanTreatsRootAsDot(t *testing.T) {
	fs := &billyPathFS{}

	tests := map[string]string{
		"":      ".",
		".":     ".",
		"/":     ".",
		"/tmp":  "tmp",
		"tmp":   "tmp",
		"/a/b/": "a/b",
	}

	for input, want := range tests {
		if got := fs.clean(input); got != want {
			t.Fatalf("clean(%q) = %q, want %q", input, got, want)
		}
	}
}
