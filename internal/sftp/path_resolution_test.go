package sftp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShouldTreatLocalGetDestinationAsDir(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	existingDir := filepath.Join(tempDir, "dest")
	if err := os.Mkdir(existingDir, 0755); err != nil {
		t.Fatalf("mkdir existing dir: %v", err)
	}

	tests := []struct {
		name        string
		destination string
		forceDir    bool
		want        bool
	}{
		{name: "forced", destination: filepath.Join(tempDir, "file.txt"), forceDir: true, want: true},
		{name: "trailing slash", destination: filepath.Join(tempDir, "newdir") + string(filepath.Separator), want: true},
		{name: "existing dir", destination: existingDir, want: true},
		{name: "plain file", destination: filepath.Join(tempDir, "file.txt"), want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldTreatLocalGetDestinationAsDir(tt.destination, tt.forceDir); got != tt.want {
				t.Fatalf("shouldTreatLocalGetDestinationAsDir(%q, %v) = %v, want %v", tt.destination, tt.forceDir, got, tt.want)
			}
		})
	}
}

func TestResolveLocalGetDestinationPath(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	tests := []struct {
		name        string
		destination string
		relpath     string
		treatAsDir  bool
		want        string
	}{
		{name: "file target", destination: filepath.Join(tempDir, "out.txt"), relpath: "src.txt", treatAsDir: false, want: filepath.Join(tempDir, "out.txt")},
		{name: "directory target", destination: filepath.Join(tempDir, "out"), relpath: filepath.Join("dir", "src.txt"), treatAsDir: true, want: filepath.Join(tempDir, "out", "dir", "src.txt")},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := resolveLocalGetDestinationPath(tt.destination, tt.relpath, tt.treatAsDir); got != tt.want {
				t.Fatalf("resolveLocalGetDestinationPath(%q, %q, %v) = %q, want %q", tt.destination, tt.relpath, tt.treatAsDir, got, tt.want)
			}
		})
	}
}

func TestResolveRemotePutDestinationPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		destination string
		relpath     string
		treatAsDir  bool
		want        string
	}{
		{name: "file target", destination: "/tmp/out.txt", relpath: "a.txt", treatAsDir: false, want: "/tmp/out.txt"},
		{name: "directory target", destination: "/tmp/out", relpath: "a.txt", treatAsDir: true, want: "/tmp/out/a.txt"},
		{name: "trailing slash", destination: "/tmp/out/", relpath: "dir/a.txt", treatAsDir: true, want: "/tmp/out/dir/a.txt"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := resolveRemotePutDestinationPath(tt.destination, tt.relpath, tt.treatAsDir); got != tt.want {
				t.Fatalf("resolveRemotePutDestinationPath(%q, %q, %v) = %q, want %q", tt.destination, tt.relpath, tt.treatAsDir, got, tt.want)
			}
		})
	}
}
