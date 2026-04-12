package sshlib

import (
	"errors"
	"os"
	"testing"
	"time"
)

type fakeRemotePathClient struct {
	home    string
	stat    os.FileInfo
	statErr error
	dirErr  error
}

func (f *fakeRemotePathClient) RealPath(path string) (string, error) {
	return f.home, nil
}

func (f *fakeRemotePathClient) Stat(path string) (os.FileInfo, error) {
	if f.statErr != nil {
		return nil, f.statErr
	}
	return f.stat, nil
}

func (f *fakeRemotePathClient) ReadDir(path string) ([]os.FileInfo, error) {
	if f.dirErr != nil {
		return nil, f.dirErr
	}
	return []os.FileInfo{}, nil
}

type fakeFileInfo struct {
	name string
	dir  bool
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { if f.dir { return os.ModeDir | 0o755 }; return 0o644 }
func (f fakeFileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (f fakeFileInfo) IsDir() bool        { return f.dir }
func (f fakeFileInfo) Sys() interface{}   { return nil }

func TestResolveRemoteBasepointRequiresExistingDirectory(t *testing.T) {
	client := &fakeRemotePathClient{
		home: "/home/test",
		stat: fakeFileInfo{name: "data", dir: true},
	}

	got, err := resolveRemoteBasepoint(client, "data")
	if err != nil {
		t.Fatalf("resolveRemoteBasepoint() error = %v", err)
	}
	if got != "/home/test/data" {
		t.Fatalf("resolveRemoteBasepoint() = %q, want %q", got, "/home/test/data")
	}
}

func TestResolveRemoteBasepointRejectsMissingPath(t *testing.T) {
	client := &fakeRemotePathClient{
		home:    "/home/test",
		statErr: errors.New("not found"),
	}

	if _, err := resolveRemoteBasepoint(client, "missing"); err == nil {
		t.Fatal("resolveRemoteBasepoint() error = nil, want error")
	}
}

func TestResolveRemoteBasepointRejectsNonDirectory(t *testing.T) {
	client := &fakeRemotePathClient{
		home: "/home/test",
		stat: fakeFileInfo{name: "file.txt", dir: false},
	}

	if _, err := resolveRemoteBasepoint(client, "file.txt"); err == nil {
		t.Fatal("resolveRemoteBasepoint() error = nil, want error")
	}
}
