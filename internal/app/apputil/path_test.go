package apputil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTransferPathSpec(t *testing.T) {
	spec, err := ParseTransferPathSpec("remote:/var/log/syslog")
	if err != nil {
		t.Fatalf("ParseTransferPathSpec() error = %v", err)
	}
	if !spec.IsRemote || spec.Path != "/var/log/syslog" {
		t.Fatalf("spec = %#v", spec)
	}
}

func TestPrepareTransferSourcePaths(t *testing.T) {
	localDir := t.TempDir()
	localFile := filepath.Join(localDir, "demo.txt")
	specs := []TransferPathSpec{
		{IsRemote: false, Path: localFile},
		{IsRemote: true, Path: "/var/lib/app data"},
	}

	if err := os.WriteFile(localFile, []byte("ok"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	paths, err := PrepareTransferSourcePaths(specs)
	if err != nil {
		t.Fatalf("PrepareTransferSourcePaths() error = %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("len(paths) = %d", len(paths))
	}
	if paths[0].Path != localFile || paths[0].DisplayPath != localFile {
		t.Fatalf("local path = %#v", paths[0])
	}
	if !strings.Contains(paths[1].Path, "\\ ") || paths[1].DisplayPath != "/var/lib/app data" {
		t.Fatalf("remote path = %#v", paths[1])
	}
}

func TestPrepareTransferDestinationPathDoesNotRequireLocalExistence(t *testing.T) {
	path, err := PrepareTransferDestinationPath(TransferPathSpec{IsRemote: false, Path: "missing/local.txt"})
	if err != nil {
		t.Fatalf("PrepareTransferDestinationPath() error = %v", err)
	}
	if !strings.HasSuffix(path.Path, filepath.FromSlash("missing/local.txt")) {
		t.Fatalf("prepared path = %#v", path)
	}
}

func TestPrepareTransferSourcePathsRejectsMissingLocalPath(t *testing.T) {
	_, err := PrepareTransferSourcePaths([]TransferPathSpec{{IsRemote: false, Path: "missing/local.txt"}})
	if err == nil || !strings.Contains(err.Error(), "not found path") {
		t.Fatalf("PrepareTransferSourcePaths() error = %v", err)
	}
}
