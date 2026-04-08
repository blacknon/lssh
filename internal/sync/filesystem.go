package sync

import (
	"io"
	"io/fs"
	"os"
	"os/user"
	pathpkg "path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
)

type FileSystem interface {
	Expand(raw string) ([]string, error)
	Resolve(raw string) (string, error)
	Stat(path string) (fs.FileInfo, error)
	Walk(root string, fn func(path string, info fs.FileInfo) error) error
	Open(path string) (io.ReadCloser, error)
	OpenWriter(path string, perm fs.FileMode) (io.WriteCloser, error)
	MkdirAll(path string) error
	Remove(path string) error
	RemoveDir(path string) error
	Chmod(path string, mode fs.FileMode) error
	Chtimes(path string, atime, mtime time.Time) error
	Clean(path string) string
	Join(elem ...string) string
	Dir(path string) string
	Separator() string
}

type localFS struct {
	cwd  string
	home string
}

func newLocalFS() (*localFS, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	usr, err := user.Current()
	if err != nil {
		return nil, err
	}

	return &localFS{
		cwd:  cwd,
		home: usr.HomeDir,
	}, nil
}

func (l *localFS) Expand(raw string) ([]string, error) {
	path, err := l.Resolve(raw)
	if err != nil {
		return nil, err
	}

	matches, err := filepath.Glob(path)
	if len(matches) == 0 {
		return []string{path}, err
	}

	return matches, err
}

func (l *localFS) Resolve(raw string) (string, error) {
	switch {
	case raw == "~":
		raw = l.home
	case strings.HasPrefix(raw, "~/"):
		raw = filepath.Join(l.home, raw[2:])
	case !filepath.IsAbs(raw):
		raw = filepath.Join(l.cwd, raw)
	}

	return filepath.Clean(raw), nil
}

func (l *localFS) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

func (l *localFS) Walk(root string, fn func(path string, info fs.FileInfo) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		return fn(path, info)
	})
}

func (l *localFS) Open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func (l *localFS) OpenWriter(path string, perm fs.FileMode) (io.WriteCloser, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
}

func (l *localFS) MkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}

func (l *localFS) Remove(path string) error {
	return os.Remove(path)
}

func (l *localFS) RemoveDir(path string) error {
	return os.Remove(path)
}

func (l *localFS) Chmod(path string, mode fs.FileMode) error {
	return os.Chmod(path, mode)
}

func (l *localFS) Chtimes(path string, atime, mtime time.Time) error {
	return os.Chtimes(path, atime, mtime)
}

func (l *localFS) Clean(path string) string {
	return filepath.Clean(path)
}

func (l *localFS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (l *localFS) Dir(path string) string {
	return filepath.Dir(path)
}

func (l *localFS) Separator() string {
	return string(filepath.Separator)
}

type remoteFS struct {
	client *sftp.Client
	pwd    string
	home   string
}

func newRemoteFS(client *sftp.Client, pwd string) *remoteFS {
	return &remoteFS{
		client: client,
		pwd:    pwd,
		home:   pwd,
	}
}

func (r *remoteFS) Expand(raw string) ([]string, error) {
	path, err := r.Resolve(raw)
	if err != nil {
		return nil, err
	}

	matches, err := r.client.Glob(path)
	if len(matches) == 0 {
		return []string{path}, err
	}

	return matches, err
}

func (r *remoteFS) Resolve(raw string) (string, error) {
	switch {
	case raw == "~":
		raw = r.home
	case strings.HasPrefix(raw, "~/"):
		raw = pathpkg.Join(r.home, raw[2:])
	case !pathpkg.IsAbs(raw):
		raw = pathpkg.Join(r.pwd, raw)
	}

	return pathpkg.Clean(raw), nil
}

func (r *remoteFS) Stat(path string) (fs.FileInfo, error) {
	return r.client.Stat(path)
}

func (r *remoteFS) Walk(root string, fn func(path string, info fs.FileInfo) error) error {
	walker := r.client.Walk(root)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return err
		}

		if err := fn(walker.Path(), walker.Stat()); err != nil {
			return err
		}
	}

	return nil
}

func (r *remoteFS) Open(path string) (io.ReadCloser, error) {
	return r.client.Open(path)
}

func (r *remoteFS) OpenWriter(path string, perm fs.FileMode) (io.WriteCloser, error) {
	return r.client.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
}

func (r *remoteFS) MkdirAll(path string) error {
	return r.client.MkdirAll(path)
}

func (r *remoteFS) Remove(path string) error {
	return r.client.Remove(path)
}

func (r *remoteFS) RemoveDir(path string) error {
	return r.client.RemoveDirectory(path)
}

func (r *remoteFS) Chmod(path string, mode fs.FileMode) error {
	return r.client.Chmod(path, mode)
}

func (r *remoteFS) Chtimes(path string, atime, mtime time.Time) error {
	return r.client.Chtimes(path, atime, mtime)
}

func (r *remoteFS) Clean(path string) string {
	return pathpkg.Clean(path)
}

func (r *remoteFS) Join(elem ...string) string {
	return pathpkg.Join(elem...)
}

func (r *remoteFS) Dir(path string) string {
	return pathpkg.Dir(path)
}

func (r *remoteFS) Separator() string {
	return "/"
}
