package absfs

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"strings"
	"syscall"
	"time"
)

// Magic header to identify overlay-created symlinks
// This distinguishes them from native symlinks if there's ever a conflict
const overlaySymlinkMagic = "ABSFS:SL:"

// SymlinkOverlay adds symlink support to filesystems that don't have native
// symlink capabilities. Symlinks are stored as regular files with:
//   - Mode: os.ModeSymlink | 0777
//   - Content: magic header + target path
//
// This allows any filesystem that stores file modes to support symlinks.
type SymlinkOverlay struct {
	base FileSystem
}

// NewSymlinkOverlay creates a SymlinkFileSystem by adding overlay-based
// symlink support to a FileSystem that doesn't have native symlinks.
//
// Note: Prefer using ExtendSymlinkFiler which automatically detects whether
// the underlying filesystem has native symlink support.
func NewSymlinkOverlay(base FileSystem) *SymlinkOverlay {
	return &SymlinkOverlay{base: base}
}

// Ensure SymlinkOverlay implements SymlinkFileSystem
var _ SymlinkFileSystem = (*SymlinkOverlay)(nil)

// Base returns the underlying FileSystem
func (o *SymlinkOverlay) Base() FileSystem {
	return o.base
}

// Symlink creates newname as a symbolic link to oldname.
func (o *SymlinkOverlay) Symlink(oldname, newname string) error {
	// Check if newname already exists
	_, err := o.base.Stat(newname)
	if err == nil {
		return &os.PathError{Op: "symlink", Path: newname, Err: syscall.EEXIST}
	}

	// Create parent directory if needed
	dir := path.Dir(newname)
	if dir != "/" && dir != "." {
		if err := o.base.MkdirAll(dir, 0755); err != nil {
			return &os.PathError{Op: "symlink", Path: newname, Err: err}
		}
	}

	// Create file with magic header + target
	f, err := o.base.Create(newname)
	if err != nil {
		return &os.PathError{Op: "symlink", Path: newname, Err: err}
	}

	_, err = f.Write([]byte(overlaySymlinkMagic + oldname))
	f.Close()
	if err != nil {
		o.base.Remove(newname)
		return &os.PathError{Op: "symlink", Path: newname, Err: err}
	}

	// Set symlink mode
	if err := o.base.Chmod(newname, os.ModeSymlink|0777); err != nil {
		o.base.Remove(newname)
		return &os.PathError{Op: "symlink", Path: newname, Err: err}
	}

	return nil
}

// Readlink returns the destination of the named symbolic link.
func (o *SymlinkOverlay) Readlink(name string) (string, error) {
	// Check it's a symlink
	info, err := o.Lstat(name)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", &os.PathError{Op: "readlink", Path: name, Err: syscall.EINVAL}
	}

	// Read file content
	content, err := o.base.ReadFile(name)
	if err != nil {
		return "", &os.PathError{Op: "readlink", Path: name,
			Err: errors.New("cannot read symlink (may be native format)")}
	}

	// Check for magic header
	if !strings.HasPrefix(string(content), overlaySymlinkMagic) {
		return "", &os.PathError{Op: "readlink", Path: name,
			Err: errors.New("symlink not created by SymlinkOverlay")}
	}

	return string(content[len(overlaySymlinkMagic):]), nil
}

// Lstat returns file info without following symlinks.
func (o *SymlinkOverlay) Lstat(name string) (os.FileInfo, error) {
	// Direct stat without symlink resolution
	return o.base.Stat(name)
}

// Lchown changes ownership of a symlink without following it.
func (o *SymlinkOverlay) Lchown(name string, uid, gid int) error {
	// For overlay symlinks, they're just files, so regular Chown works
	return o.base.Chown(name, uid, gid)
}

// resolvePath resolves symlinks in a path, returning the final resolved path.
// maxDepth prevents infinite loops from circular symlinks.
func (o *SymlinkOverlay) resolvePath(p string, maxDepth int) (string, error) {
	if maxDepth <= 0 {
		return "", syscall.ELOOP
	}

	p = path.Clean(p)
	if p == "/" {
		return "/", nil
	}

	// Split into components and resolve each
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	resolved := "/"

	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			resolved = path.Dir(resolved)
			continue
		}

		candidate := path.Join(resolved, part)

		// Check if this component is a symlink
		info, err := o.base.Stat(candidate)
		if err != nil {
			// Path doesn't exist yet - that's fine for the last component
			resolved = candidate
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			// It's a symlink - read and resolve target
			target, err := o.Readlink(candidate)
			if err != nil {
				return "", err
			}

			// Resolve relative vs absolute target
			var newPath string
			if path.IsAbs(target) {
				newPath = target
			} else {
				newPath = path.Join(resolved, target)
			}

			// Recursively resolve
			resolved, err = o.resolvePath(newPath, maxDepth-1)
			if err != nil {
				return "", err
			}
		} else {
			resolved = candidate
		}
	}

	return resolved, nil
}

// resolve is a convenience wrapper with default max depth
func (o *SymlinkOverlay) resolve(p string) (string, error) {
	return o.resolvePath(p, 40) // Same as Linux MAXSYMLINKS
}

// FileSystem interface implementation - with symlink resolution

func (o *SymlinkOverlay) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	resolved, err := o.resolve(name)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	return o.base.OpenFile(resolved, flag, perm)
}

func (o *SymlinkOverlay) Mkdir(name string, perm os.FileMode) error {
	resolved, err := o.resolve(path.Dir(name))
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: err}
	}
	return o.base.Mkdir(path.Join(resolved, path.Base(name)), perm)
}

func (o *SymlinkOverlay) Remove(name string) error {
	// Remove doesn't follow symlinks - removes the symlink itself
	return o.base.Remove(name)
}

func (o *SymlinkOverlay) Rename(oldpath, newpath string) error {
	// Rename doesn't follow symlinks
	return o.base.Rename(oldpath, newpath)
}

func (o *SymlinkOverlay) Stat(name string) (os.FileInfo, error) {
	resolved, err := o.resolve(name)
	if err != nil {
		return nil, &os.PathError{Op: "stat", Path: name, Err: err}
	}
	info, err := o.base.Stat(resolved)
	if err != nil {
		return nil, err
	}
	// Return info with original name
	return &renamedFileInfo{info, path.Base(name)}, nil
}

func (o *SymlinkOverlay) Chmod(name string, mode os.FileMode) error {
	resolved, err := o.resolve(name)
	if err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: err}
	}
	return o.base.Chmod(resolved, mode)
}

func (o *SymlinkOverlay) Chtimes(name string, atime, mtime time.Time) error {
	resolved, err := o.resolve(name)
	if err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: err}
	}
	return o.base.Chtimes(resolved, atime, mtime)
}

func (o *SymlinkOverlay) Chown(name string, uid, gid int) error {
	resolved, err := o.resolve(name)
	if err != nil {
		return &os.PathError{Op: "chown", Path: name, Err: err}
	}
	return o.base.Chown(resolved, uid, gid)
}

// Convenience methods from FileSystem

func (o *SymlinkOverlay) Open(name string) (File, error) {
	return o.OpenFile(name, os.O_RDONLY, 0)
}

func (o *SymlinkOverlay) Create(name string) (File, error) {
	return o.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
}

func (o *SymlinkOverlay) MkdirAll(p string, perm os.FileMode) error {
	return o.base.MkdirAll(p, perm)
}

func (o *SymlinkOverlay) RemoveAll(name string) error {
	return o.base.RemoveAll(name)
}

func (o *SymlinkOverlay) Truncate(name string, size int64) error {
	resolved, err := o.resolve(name)
	if err != nil {
		return &os.PathError{Op: "truncate", Path: name, Err: err}
	}
	return o.base.Truncate(resolved, size)
}

func (o *SymlinkOverlay) ReadDir(name string) ([]fs.DirEntry, error) {
	resolved, err := o.resolve(name)
	if err != nil {
		return nil, &os.PathError{Op: "readdir", Path: name, Err: err}
	}
	return o.base.ReadDir(resolved)
}

func (o *SymlinkOverlay) ReadFile(name string) ([]byte, error) {
	resolved, err := o.resolve(name)
	if err != nil {
		return nil, &os.PathError{Op: "readfile", Path: name, Err: err}
	}
	return o.base.ReadFile(resolved)
}

func (o *SymlinkOverlay) Sub(dir string) (fs.FS, error) {
	return o.base.Sub(dir)
}

func (o *SymlinkOverlay) Chdir(dir string) error {
	return o.base.Chdir(dir)
}

func (o *SymlinkOverlay) Getwd() (string, error) {
	return o.base.Getwd()
}

func (o *SymlinkOverlay) TempDir() string {
	return o.base.TempDir()
}

// renamedFileInfo wraps FileInfo to return a different name
type renamedFileInfo struct {
	os.FileInfo
	name string
}

func (r *renamedFileInfo) Name() string {
	return r.name
}
