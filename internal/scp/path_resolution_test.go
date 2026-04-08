package scp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShouldTreatLocalDestinationAsDir(t *testing.T) {
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

			if got := shouldTreatLocalDestinationAsDir(tt.destination, tt.forceDir); got != tt.want {
				t.Fatalf("shouldTreatLocalDestinationAsDir(%q, %v) = %v, want %v", tt.destination, tt.forceDir, got, tt.want)
			}
		})
	}
}

func TestCopySourceBase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		source             string
		sourceIsDir        bool
		preserveSourceName bool
		want               string
	}{
		{name: "file", source: filepath.Join("tmp", "a.txt"), want: "tmp"},
		{name: "directory preserved", source: filepath.Join("tmp", "src"), sourceIsDir: true, preserveSourceName: true, want: "tmp"},
		{name: "directory renamed", source: filepath.Join("tmp", "src"), sourceIsDir: true, want: filepath.Join("tmp", "src")},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := copySourceBase(tt.source, tt.sourceIsDir, tt.preserveSourceName); got != tt.want {
				t.Fatalf("copySourceBase(%q, %v, %v) = %q, want %q", tt.source, tt.sourceIsDir, tt.preserveSourceName, got, tt.want)
			}
		})
	}
}

func TestResolveRemoteDestinationPath(t *testing.T) {
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
		{name: "root element", destination: "/tmp/out", relpath: ".", treatAsDir: true, want: "/tmp/out"},
		{name: "trailing slash", destination: "/tmp/out/", relpath: "dir/a.txt", treatAsDir: true, want: "/tmp/out/dir/a.txt"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := resolveRemoteDestinationPath(tt.destination, tt.relpath, tt.treatAsDir); got != tt.want {
				t.Fatalf("resolveRemoteDestinationPath(%q, %q, %v) = %q, want %q", tt.destination, tt.relpath, tt.treatAsDir, got, tt.want)
			}
		})
	}
}

func TestResolveLocalDestinationPath(t *testing.T) {
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

			if got := resolveLocalDestinationPath(tt.destination, tt.relpath, tt.treatAsDir); got != tt.want {
				t.Fatalf("resolveLocalDestinationPath(%q, %q, %v) = %q, want %q", tt.destination, tt.relpath, tt.treatAsDir, got, tt.want)
			}
		})
	}
}

func TestDirectoryCopyDestinationSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		source             string
		current            string
		sourceIsDir        bool
		preserveSourceName bool
		destination        string
		want               string
	}{
		{
			name:               "existing destination directory keeps source name",
			source:             "/src/tmp",
			current:            "/src/tmp/file.txt",
			sourceIsDir:        true,
			preserveSourceName: true,
			destination:        "/tmp",
			want:               "/tmp/tmp/file.txt",
		},
		{
			name:        "single directory to new destination renames root",
			source:      "/src/tmp",
			current:     "/src/tmp/file.txt",
			sourceIsDir: true,
			destination: "/out",
			want:        "/out/file.txt",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			base := copySourceBase(tt.source, tt.sourceIsDir, tt.preserveSourceName)
			relpath, err := filepath.Rel(base, tt.current)
			if err != nil {
				t.Fatalf("filepath.Rel(%q, %q): %v", base, tt.current, err)
			}

			got := resolveLocalDestinationPath(tt.destination, relpath, tt.sourceIsDir || tt.preserveSourceName)
			if got != tt.want {
				t.Fatalf("resolved destination = %q, want %q", got, tt.want)
			}
		})
	}
}
