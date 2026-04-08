package sync

import (
	"io"
	"io/fs"
	"os"
	pathpkg "path"
	"sort"
	"strings"
	"testing"
	"time"
)

type fakeInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

func (f fakeInfo) Name() string       { return f.name }
func (f fakeInfo) Size() int64        { return f.size }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return f.modTime }
func (f fakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeInfo) Sys() interface{}   { return nil }

type fakeFS struct {
	entries map[string]fakeInfo
	expands map[string][]string
}

func newFakeFS() *fakeFS {
	return &fakeFS{
		entries: map[string]fakeInfo{},
		expands: map[string][]string{},
	}
}

func (f *fakeFS) addFile(path string, size int64, mod time.Time) {
	f.entries[path] = fakeInfo{name: pathpkg.Base(path), size: size, mode: 0644, modTime: mod}
}

func (f *fakeFS) addDir(path string, mod time.Time) {
	f.entries[path] = fakeInfo{name: pathpkg.Base(path), mode: fs.ModeDir | 0755, modTime: mod}
}

func (f *fakeFS) Expand(raw string) ([]string, error) {
	if expand, ok := f.expands[raw]; ok {
		return expand, nil
	}
	return []string{raw}, nil
}

func (f *fakeFS) Resolve(raw string) (string, error) { return pathpkg.Clean(raw), nil }
func (f *fakeFS) Stat(path string) (fs.FileInfo, error) {
	info, ok := f.entries[pathpkg.Clean(path)]
	if !ok {
		return nil, os.ErrNotExist
	}
	return info, nil
}

func (f *fakeFS) Walk(root string, fn func(path string, info fs.FileInfo) error) error {
	root = pathpkg.Clean(root)
	keys := []string{}
	for path := range f.entries {
		if path == root || strings.HasPrefix(path, root+"/") {
			keys = append(keys, path)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := fn(key, f.entries[key]); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeFS) Open(path string) (io.ReadCloser, error)                          { return nil, nil }
func (f *fakeFS) OpenWriter(path string, perm fs.FileMode) (io.WriteCloser, error) { return nil, nil }
func (f *fakeFS) MkdirAll(path string) error                                       { return nil }
func (f *fakeFS) Remove(path string) error                                         { return nil }
func (f *fakeFS) RemoveDir(path string) error                                      { return nil }
func (f *fakeFS) Chmod(path string, mode fs.FileMode) error                        { return nil }
func (f *fakeFS) Chtimes(path string, atime, mtime time.Time) error                { return nil }
func (f *fakeFS) Clean(path string) string                                         { return pathpkg.Clean(path) }
func (f *fakeFS) Join(elem ...string) string                                       { return pathpkg.Join(elem...) }
func (f *fakeFS) Dir(path string) string                                           { return pathpkg.Dir(path) }
func (f *fakeFS) Separator() string                                                { return "/" }

func TestBuildPlanSingleDirectoryRename(t *testing.T) {
	t.Parallel()

	src := newFakeFS()
	dst := newFakeFS()
	now := time.Unix(100, 0)
	src.addDir("/src/app", now)
	src.addFile("/src/app/file.txt", 10, now)

	plan, err := BuildPlan(src, dst, []string{"/src/app"}, "/deploy")
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if _, ok := plan.Desired["/deploy/file.txt"]; !ok {
		t.Fatalf("expected /deploy/file.txt to be planned")
	}
	if _, ok := plan.Desired["/deploy"]; !ok {
		t.Fatalf("expected /deploy directory to be planned")
	}
	if len(plan.DeleteScopes) != 1 || plan.DeleteScopes[0].Path != "/deploy" {
		t.Fatalf("unexpected delete scopes: %#v", plan.DeleteScopes)
	}
}

func TestBuildPlanExistingDirectoryKeepsSourceName(t *testing.T) {
	t.Parallel()

	src := newFakeFS()
	dst := newFakeFS()
	now := time.Unix(100, 0)
	src.addDir("/src/app", now)
	src.addFile("/src/app/file.txt", 10, now)
	dst.addDir("/tmp", now)

	plan, err := BuildPlan(src, dst, []string{"/src/app"}, "/tmp")
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if _, ok := plan.Desired["/tmp/app/file.txt"]; !ok {
		t.Fatalf("expected /tmp/app/file.txt to be planned")
	}
	if len(plan.DeleteScopes) != 1 || plan.DeleteScopes[0].Path != "/tmp/app" {
		t.Fatalf("unexpected delete scopes: %#v", plan.DeleteScopes)
	}
}

func TestBuildPlanSameNamedExistingDestinationDoesNotNest(t *testing.T) {
	t.Parallel()

	src := newFakeFS()
	dst := newFakeFS()
	now := time.Unix(100, 0)
	src.addDir("/src/hoge", now)
	src.addFile("/src/hoge/file.txt", 10, now)
	dst.addDir("/tmp/hoge", now)

	plan, err := BuildPlan(src, dst, []string{"/src/hoge"}, "/tmp/hoge")
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if _, ok := plan.Desired["/tmp/hoge/file.txt"]; !ok {
		t.Fatalf("expected /tmp/hoge/file.txt to be planned")
	}
	if _, ok := plan.Desired["/tmp/hoge/hoge/file.txt"]; ok {
		t.Fatalf("did not expect nested /tmp/hoge/hoge/file.txt")
	}
	if len(plan.DeleteScopes) != 1 || plan.DeleteScopes[0].Path != "/tmp/hoge" {
		t.Fatalf("unexpected delete scopes: %#v", plan.DeleteScopes)
	}
}

func TestBuildPlanMultipleSourcesUseSharedDeleteScope(t *testing.T) {
	t.Parallel()

	src := newFakeFS()
	dst := newFakeFS()
	now := time.Unix(100, 0)
	src.addFile("/src/a.txt", 1, now)
	src.addFile("/src/b.txt", 2, now)

	plan, err := BuildPlan(src, dst, []string{"/src/a.txt", "/src/b.txt"}, "/dest")
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if len(plan.DeleteScopes) != 1 || plan.DeleteScopes[0].Path != "/dest" || !plan.DeleteScopes[0].IsDir {
		t.Fatalf("unexpected delete scopes: %#v", plan.DeleteScopes)
	}
	if _, ok := plan.Desired["/dest/a.txt"]; !ok {
		t.Fatalf("expected /dest/a.txt to be planned")
	}
	if _, ok := plan.Desired["/dest/b.txt"]; !ok {
		t.Fatalf("expected /dest/b.txt to be planned")
	}
}

func TestPathsToDelete(t *testing.T) {
	t.Parallel()

	desired := map[string]DesiredEntry{
		"/dest/app/file.txt": {DestinationPath: "/dest/app/file.txt"},
	}
	scope := DeleteScope{Path: "/dest/app", IsDir: true}
	existing := []string{
		"/dest/app/file.txt",
		"/dest/app/old.txt",
		"/dest/app/extra",
		"/dest/app/extra/nested.txt",
	}

	got := pathsToDelete(scope, existing, desired, pathpkg.Clean, pathpkg.Dir, "/")
	want := []string{
		"/dest/app/extra/nested.txt",
		"/dest/app/old.txt",
		"/dest/app/extra",
	}

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("pathsToDelete() = %v, want %v", got, want)
	}
}

func TestFileNeedsCopy(t *testing.T) {
	t.Parallel()

	dst := newFakeFS()
	now := time.Unix(100, 0)
	dst.addFile("/dest/file.txt", 10, now)

	needsCopy, err := fileNeedsCopy(dst, DesiredEntry{
		DestinationPath: "/dest/file.txt",
		Size:            10,
		ModTime:         now,
	})
	if err != nil {
		t.Fatalf("fileNeedsCopy returned error: %v", err)
	}
	if needsCopy {
		t.Fatalf("expected identical file to be skipped")
	}

	needsCopy, err = fileNeedsCopy(dst, DesiredEntry{
		DestinationPath: "/dest/file.txt",
		Size:            11,
		ModTime:         now,
	})
	if err != nil {
		t.Fatalf("fileNeedsCopy returned error: %v", err)
	}
	if !needsCopy {
		t.Fatalf("expected changed file to require copy")
	}
}
