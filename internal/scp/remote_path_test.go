package scp

import (
	"io/fs"
	"testing"
	"time"
)

type stubFileInfo struct {
	dir bool
}

func (s stubFileInfo) Name() string       { return "stub" }
func (s stubFileInfo) Size() int64        { return 0 }
func (s stubFileInfo) Mode() fs.FileMode  { return 0 }
func (s stubFileInfo) ModTime() time.Time { return time.Time{} }
func (s stubFileInfo) IsDir() bool        { return s.dir }
func (s stubFileInfo) Sys() interface{}   { return nil }

func TestRemoteFileInfoIsDirectory(t *testing.T) {
	t.Parallel()

	if !remoteFileInfoIsDirectory(stubFileInfo{dir: true}) {
		t.Fatal("expected directory to be detected")
	}
	if remoteFileInfoIsDirectory(stubFileInfo{dir: false}) {
		t.Fatal("expected regular file to not be treated as directory")
	}
	if remoteFileInfoIsDirectory(nil) {
		t.Fatal("expected nil file info to not be treated as directory")
	}
}
