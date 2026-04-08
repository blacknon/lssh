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

func TestDirectoryTransferDestinationSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		source             string
		current            string
		sourceIsDir        bool
		preserveSourceName bool
		destination        string
		wantLocal          string
		wantRemote         string
	}{
		{
			name:               "existing destination directory keeps source name",
			source:             "/src/tmp",
			current:            "/src/tmp/file.txt",
			sourceIsDir:        true,
			preserveSourceName: true,
			destination:        "/tmp",
			wantLocal:          filepath.Join(string(filepath.Separator), "tmp", "tmp", "file.txt"),
			wantRemote:         "/tmp/tmp/file.txt",
		},
		{
			name:        "single directory to new destination renames root",
			source:      "/src/tmp",
			current:     "/src/tmp/file.txt",
			sourceIsDir: true,
			destination: "/out",
			wantLocal:   filepath.Join(string(filepath.Separator), "out", "file.txt"),
			wantRemote:  "/out/file.txt",
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

			localGot := resolveLocalGetDestinationPath(tt.destination, relpath, tt.sourceIsDir || tt.preserveSourceName)
			if localGot != tt.wantLocal {
				t.Fatalf("resolveLocalGetDestinationPath(...) = %q, want %q", localGot, tt.wantLocal)
			}

			remoteGot := resolveRemotePutDestinationPath(tt.destination, relpath, tt.sourceIsDir || tt.preserveSourceName)
			if remoteGot != tt.wantRemote {
				t.Fatalf("resolveRemotePutDestinationPath(...) = %q, want %q", remoteGot, tt.wantRemote)
			}
		})
	}
}
