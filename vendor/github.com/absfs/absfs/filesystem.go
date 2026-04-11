package absfs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"
)

// Path separators for absfs virtual filesystems.
// All absfs filesystems use Unix-style paths regardless of host OS.
const (
	Separator     = '/'
	ListSeparator = ':'
)

var ErrNotImplemented = errors.New("not implemented")

// toSlash normalizes a path to always use forward slashes.
// This is critical for cross-platform compatibility since all absfs
// virtual filesystems use Unix-style forward slash paths.
func toSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

type ReadOnlyFiler interface {
	Open(name string) (io.ReadCloser, error)
}

type WriteOnlyFiler interface {
	Open(name string) (io.WriteCloser, error)
}

type Filer interface {

	// OpenFile opens a file using the given flags and the given mode.
	OpenFile(name string, flag int, perm os.FileMode) (File, error)

	// Mkdir creates a directory in the filesystem, return an error if any
	// happens.
	Mkdir(name string, perm os.FileMode) error

	// Remove removes a file identified by name, returning an error, if any
	// happens.
	Remove(name string) error

	// Rename renames (moves) oldpath to newpath. If newpath already exists and
	// is not a directory, Rename replaces it. OS-specific restrictions may apply
	// when oldpath and newpath are in different directories. If there is an
	// error, it will be of type *LinkError.
	Rename(oldpath, newpath string) error

	// Stat returns the FileInfo structure describing file. If there is an error,
	// it will be of type *PathError.
	Stat(name string) (os.FileInfo, error)

	//Chmod changes the mode of the named file to mode.
	Chmod(name string, mode os.FileMode) error

	//Chtimes changes the access and modification times of the named file
	Chtimes(name string, atime time.Time, mtime time.Time) error

	//Chown changes the owner and group ids of the named file
	Chown(name string, uid, gid int) error

	// ReadDir reads the named directory and returns a list of directory entries
	// sorted by filename. This is compatible with io/fs.ReadDirFS.
	ReadDir(name string) ([]fs.DirEntry, error)

	// ReadFile reads the named file and returns its contents.
	// This is compatible with io/fs.ReadFileFS.
	ReadFile(name string) ([]byte, error)

	// Sub returns an fs.FS corresponding to the subtree rooted at dir.
	// This is compatible with io/fs.SubFS interface.
	// The returned fs.FS is read-only; for writable subdirectory access, use basefs.
	Sub(dir string) (fs.FS, error)
}

type FileSystem interface {
	Filer

	Chdir(dir string) error
	Getwd() (dir string, err error)
	TempDir() string
	Open(name string) (File, error)
	Create(name string) (File, error)
	MkdirAll(name string, perm os.FileMode) error
	RemoveAll(path string) (err error)
	Truncate(name string, size int64) error
}

type SymLinker interface {
	// SameFile(fi1, fi2 os.FileInfo) bool

	// Lstat returns a FileInfo describing the named file. If the file is a
	// symbolic link, the returned FileInfo describes the symbolic link. Lstat
	// makes no attempt to follow the link. If there is an error, it will be of type *PathError.
	Lstat(name string) (os.FileInfo, error)

	// Lchown changes the numeric uid and gid of the named file. If the file is a
	// symbolic link, it changes the uid and gid of the link itself. If there is
	// an error, it will be of type *PathError.
	//
	// On Windows, it always returns the syscall.EWINDOWS error, wrapped in
	// *PathError.
	Lchown(name string, uid, gid int) error

	// Readlink returns the destination of the named symbolic link. If there is an
	// error, it will be of type *PathError.
	Readlink(name string) (string, error)

	// Symlink creates newname as a symbolic link to oldname. If there is an
	// error, it will be of type *LinkError.
	Symlink(oldname, newname string) error
}

type SymlinkFileSystem interface {
	FileSystem
	SymLinker
}

// ExtendFiler adds the FileSystem convenience functions to any Filer implementation.
//
// Path Semantics:
//
// All absfs filesystems use Unix-style forward slash paths ("/") on all platforms.
// This design enables virtual filesystems (mocks, in-memory, archives) to work
// consistently across all platforms and compose seamlessly with each other.
//
// Examples:
//   - "/config/app.json"       → absolute path on all platforms
//   - "/home/user/data"        → absolute path on all platforms
//   - "relative/path"          → relative on all platforms
//
// For OS filesystem wrappers (like osfs), use the osfs.ToNative() and
// osfs.FromNative() helpers to convert between absfs paths and native OS paths.
//
// See PATH_HANDLING.md for detailed cross-platform behavior documentation.
func ExtendFiler(filer Filer) FileSystem {
	return &extendedFS{"/", filer}
}

type extendedFS struct {
	cwd   string
	filer Filer
}

// Compile-time check that extendedFS implements FileSystem
var _ FileSystem = (*extendedFS)(nil)

// Compile-time check that extendedFS implements SymLinker
// (it always has the methods, but they only work if the underlying Filer supports symlinks)
var _ SymLinker = (*extendedFS)(nil)

// isVirtualAbs checks if a path should be treated as absolute in the virtual filesystem.
// For virtual filesystems, paths starting with '/' are absolute.
func isVirtualAbs(p string) bool {
	return path.IsAbs(p)
}

func (efs *extendedFS) OpenFile(name string, flag int, perm os.FileMode) (f File, err error) {
	name = toSlash(name)
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}
	return efs.filer.OpenFile(name, flag, perm)
}

func (efs *extendedFS) Mkdir(name string, perm os.FileMode) error {
	name = toSlash(name)
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}
	return efs.filer.Mkdir(name, perm)
}

func (efs *extendedFS) Remove(name string) error {
	name = toSlash(name)
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}
	return efs.filer.Remove(name)
}

func (efs *extendedFS) Rename(oldpath, newpath string) error {
	oldpath = toSlash(oldpath)
	newpath = toSlash(newpath)
	if !isVirtualAbs(oldpath) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			oldpath = path.Join(efs.cwd, oldpath)
		}
	}
	if !isVirtualAbs(newpath) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			newpath = path.Join(efs.cwd, newpath)
		}
	}

	return efs.filer.Rename(oldpath, newpath)
}

func (efs *extendedFS) Stat(name string) (os.FileInfo, error) {
	name = toSlash(name)
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}
	return efs.filer.Stat(name)
}

func (efs *extendedFS) Chmod(name string, mode os.FileMode) error {
	name = toSlash(name)
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}
	return efs.filer.Chmod(name, mode)
}

func (efs *extendedFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	name = toSlash(name)
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}

	return efs.filer.Chtimes(name, atime, mtime)
}

func (efs *extendedFS) Chown(name string, uid, gid int) error {
	name = toSlash(name)
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}
	return efs.filer.Chown(name, uid, gid)
}


func (efs *extendedFS) Chdir(dir string) error {
	// Normalize to forward slashes for cross-platform compatibility
	dir = toSlash(dir)

	if filer, ok := efs.filer.(dirnavigator); ok {
		return filer.Chdir(dir)
	}
	f, err := efs.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return &os.PathError{Op: "chdir", Path: dir, Err: errors.New("not a directory")}
	}
	efs.cwd = path.Clean(dir)
	return nil
}

func (efs *extendedFS) Getwd() (dir string, err error) {
	if filer, ok := efs.filer.(dirnavigator); ok {
		return filer.Getwd()
	}
	return efs.cwd, nil
}

func (efs *extendedFS) TempDir() string {
	if filer, ok := efs.filer.(temper); ok {
		return filer.TempDir()
	}

	return "/tmp"
}

func (efs *extendedFS) Open(name string) (File, error) {
	name = toSlash(name)
	if filer, ok := efs.filer.(opener); ok {
		return filer.Open(name)
	}
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}
	return efs.filer.OpenFile(name, os.O_RDONLY, 0)
}

func (efs *extendedFS) Create(name string) (File, error) {
	name = toSlash(name)
	if filer, ok := efs.filer.(creator); ok {
		return filer.Create(name)
	}
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}
	return efs.filer.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
}

func (efs *extendedFS) MkdirAll(name string, perm os.FileMode) error {
	name = toSlash(name)
	if filer, ok := efs.filer.(mkaller); ok {
		return filer.MkdirAll(name, perm)
	}
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}

	// Clean the path using Unix-style path cleaning
	name = path.Clean(name)

	// Build directories one at a time
	p := "/"
	for _, component := range strings.Split(name, "/") {
		if component == "" {
			continue
		}
		p = path.Join(p, component)
		if p == "/" {
			continue
		}
		efs.Mkdir(p, perm)
	}

	return nil
}

func (efs *extendedFS) removeAll(p string) error {
	// open the file to check if it's a directory
	f, err := efs.Open(p)
	if err != nil {
		return err
	}

	// get FileInfo
	info, err := f.Stat()
	closeErr := f.Close()
	if err != nil {
		return err
	}

	// if it's not a directory, just remove it
	if !info.IsDir() {
		return efs.Remove(p)
	}

	// For directories, we need to recursively remove contents
	// Reopen to read directory entries
	f, err = efs.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()

	// Read all directory entries and remove them recursively
	for {
		names, err := f.Readdirnames(512)
		for _, name := range names {
			if name == "." || name == ".." {
				continue
			}
			if err := efs.removeAll(path.Join(p, name)); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// Close the directory before removing it
	if err := f.Close(); err != nil && closeErr == nil {
		closeErr = err
	}

	// Finally, remove the directory itself
	if err := efs.filer.Remove(p); err != nil {
		return err
	}

	return closeErr
}

func (efs *extendedFS) RemoveAll(name string) (err error) {
	name = toSlash(name)
	if filer, ok := efs.filer.(remover); ok {
		return filer.RemoveAll(name)
	}
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}
	return efs.removeAll(name)
}

func (efs *extendedFS) Truncate(name string, size int64) error {
	name = toSlash(name)
	if filer, ok := efs.filer.(truncater); ok {
		return filer.Truncate(name, size)
	}
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}

	f, err := efs.filer.OpenFile(name, os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	return f.Close()
}

// ReadDir reads the named directory and returns a list of directory entries.
// If the underlying Filer implements ReadDir, it delegates to that implementation.
// Otherwise, it uses a fallback implementation via Open and File.ReadDir.
func (efs *extendedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	name = toSlash(name)
	if filer, ok := efs.filer.(dirReader); ok {
		return filer.ReadDir(name)
	}
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}

	// Fallback: Open dir, call File.ReadDir(-1), close
	f, err := efs.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.ReadDir(-1)
}

// ReadFile reads the named file and returns its contents.
// If the underlying Filer implements ReadFile, it delegates to that implementation.
// Otherwise, it uses a fallback implementation via Open and Read.
func (efs *extendedFS) ReadFile(name string) ([]byte, error) {
	name = toSlash(name)
	if filer, ok := efs.filer.(fileReader); ok {
		return filer.ReadFile(name)
	}
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}

	// Fallback: Open, Stat for size hint, Read all, Close
	f, err := efs.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Get size hint for efficient allocation
	var size int64
	if info, err := f.Stat(); err == nil {
		size = info.Size()
	}

	// Pre-allocate buffer if we have size info
	if size > 0 {
		buf := make([]byte, size)
		n, err := io.ReadFull(f, buf)
		if err == io.ErrUnexpectedEOF {
			// File was shorter than expected, return what we got
			return buf[:n], nil
		}
		if err != nil {
			return nil, err
		}
		return buf, nil
	}

	// Unknown size, read all
	return io.ReadAll(f)
}

// Sub returns an fs.FS corresponding to the subtree rooted at dir.
// If the underlying Filer implements Sub, it delegates to that implementation.
// Otherwise, it uses FilerToFS to create a read-only fs.FS wrapper.
func (efs *extendedFS) Sub(dir string) (fs.FS, error) {
	dir = toSlash(dir)
	if filer, ok := efs.filer.(subber); ok {
		return filer.Sub(dir)
	}
	if !isVirtualAbs(dir) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			dir = path.Join(efs.cwd, dir)
		}
	}

	return FilerToFS(efs.filer, dir)
}

// FilerToFS creates an fs.FS from any Filer, rooted at the given directory.
// This is the canonical way to create an fs.FS from absfs types.
// The returned fs.FS is read-only; all opens use O_RDONLY.
func FilerToFS(filer Filer, root string) (fs.FS, error) {
	root = path.Clean(root)

	// Validate directory exists
	info, err := filer.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &os.PathError{Op: "sub", Path: root, Err: errors.New("not a directory")}
	}

	return &subFS{filer: filer, root: root}, nil
}

// subFS implements fs.FS by wrapping a Filer (read-only).
// It validates paths per fs.ValidPath rules and enforces read-only access.
type subFS struct {
	filer Filer
	root  string
}

// Open opens the named file for reading.
// The name must be a valid fs.FS path (no "..", no absolute paths, clean).
func (s *subFS) Open(name string) (fs.File, error) {
	// Validate path per fs.ValidPath rules
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	// Construct full path
	fullPath := name
	if s.root != "." && s.root != "" {
		fullPath = path.Join(s.root, name)
	}

	// Open read-only - enforces fs.FS read-only contract
	file, err := s.filer.OpenFile(fullPath, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	// absfs.File already satisfies fs.File
	return file, nil
}

// Sub returns an fs.FS for a subdirectory within this subFS.
// This implements fs.SubFS for nested Sub calls.
func (s *subFS) Sub(dir string) (fs.FS, error) {
	// Validate path per fs.ValidPath rules
	if !fs.ValidPath(dir) {
		return nil, &fs.PathError{Op: "sub", Path: dir, Err: fs.ErrInvalid}
	}

	// Construct full path
	fullPath := dir
	if s.root != "." && s.root != "" {
		fullPath = path.Join(s.root, dir)
	}

	return FilerToFS(s.filer, fullPath)
}

// ReadDir reads and returns the directory entries for the named directory.
// This implements fs.ReadDirFS.
func (s *subFS) ReadDir(name string) ([]fs.DirEntry, error) {
	// Validate path per fs.ValidPath rules
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}

	// Construct full path
	fullPath := name
	if s.root != "." && s.root != "" {
		fullPath = path.Join(s.root, name)
	}

	return s.filer.ReadDir(fullPath)
}

// ReadFile reads and returns the contents of the named file.
// This implements fs.ReadFileFS.
func (s *subFS) ReadFile(name string) ([]byte, error) {
	// Validate path per fs.ValidPath rules
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrInvalid}
	}

	// Construct full path
	fullPath := name
	if s.root != "." && s.root != "" {
		fullPath = path.Join(s.root, name)
	}

	return s.filer.ReadFile(fullPath)
}

// Stat returns the FileInfo for the named file.
// This implements fs.StatFS.
func (s *subFS) Stat(name string) (fs.FileInfo, error) {
	// Validate path per fs.ValidPath rules
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}

	// Construct full path
	fullPath := name
	if s.root != "." && s.root != "" {
		fullPath = path.Join(s.root, name)
	}

	return s.filer.Stat(fullPath)
}

// subFiler wraps a FileSystem and prefixes all paths with a directory.
// NOTE: subFiler is used internally for writable subtree access.
// For read-only fs.FS access, use FilerToFS or the Sub method.
type subFiler struct {
	efs *extendedFS
	dir string
}

func (s *subFiler) fullPath(name string) string {
	name = toSlash(name)
	// Prevent escaping the subtree
	clean := path.Clean("/" + name)
	return path.Join(s.dir, clean)
}

func (s *subFiler) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	return s.efs.OpenFile(s.fullPath(name), flag, perm)
}

func (s *subFiler) Mkdir(name string, perm os.FileMode) error {
	return s.efs.Mkdir(s.fullPath(name), perm)
}

func (s *subFiler) Remove(name string) error {
	return s.efs.Remove(s.fullPath(name))
}

func (s *subFiler) Rename(oldpath, newpath string) error {
	return s.efs.Rename(s.fullPath(oldpath), s.fullPath(newpath))
}

func (s *subFiler) Stat(name string) (os.FileInfo, error) {
	return s.efs.Stat(s.fullPath(name))
}

func (s *subFiler) Chmod(name string, mode os.FileMode) error {
	return s.efs.Chmod(s.fullPath(name), mode)
}

func (s *subFiler) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return s.efs.Chtimes(s.fullPath(name), atime, mtime)
}

func (s *subFiler) Chown(name string, uid, gid int) error {
	return s.efs.Chown(s.fullPath(name), uid, gid)
}

func (s *subFiler) ReadDir(name string) ([]fs.DirEntry, error) {
	return s.efs.ReadDir(s.fullPath(name))
}

func (s *subFiler) ReadFile(name string) ([]byte, error) {
	return s.efs.ReadFile(s.fullPath(name))
}

func (s *subFiler) Sub(dir string) (fs.FS, error) {
	fullDir := s.fullPath(dir)
	return FilerToFS(s.efs.filer, fullDir)
}


// interfaces for easy method typing

type opener interface {
	Open(name string) (File, error)
}

type creator interface {
	Create(name string) (File, error)
}

type mkaller interface {
	MkdirAll(name string, perm os.FileMode) error
}

type remover interface {
	RemoveAll(path string) (err error)
}

type dirReader interface {
	ReadDir(name string) ([]fs.DirEntry, error)
}

type fileReader interface {
	ReadFile(name string) ([]byte, error)
}

type subber interface {
	Sub(dir string) (fs.FS, error)
}

type dirnavigator interface {
	Chdir(dir string) error
	Getwd() (dir string, err error)
}

type temper interface {
	TempDir() string
}

type truncater interface {
	Truncate(name string, size int64) error
}

// symlinker is the internal interface for checking SymLinker capability.
// Named differently from the exported SymLinker to avoid confusion.
type symlinker interface {
	Lstat(name string) (os.FileInfo, error)
	Lchown(name string, uid, gid int) error
	Readlink(name string) (string, error)
	Symlink(oldname, newname string) error
}

// Lstat returns file info without following symlinks.
// If the underlying Filer implements SymLinker, it delegates to that implementation.
// Otherwise, it falls back to Stat (which may follow symlinks).
func (efs *extendedFS) Lstat(name string) (os.FileInfo, error) {
	name = toSlash(name)
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}
	if linker, ok := efs.filer.(symlinker); ok {
		return linker.Lstat(name)
	}
	// Fallback to Stat
	return efs.filer.Stat(name)
}

// Lchown changes the ownership of a symlink (without following it).
// If the underlying Filer implements SymLinker, it delegates to that implementation.
// Otherwise, it returns an error.
func (efs *extendedFS) Lchown(name string, uid, gid int) error {
	name = toSlash(name)
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}
	if linker, ok := efs.filer.(symlinker); ok {
		return linker.Lchown(name, uid, gid)
	}
	return &os.PathError{Op: "lchown", Path: name, Err: errors.New("symlinks not supported")}
}

// Readlink returns the destination of a symlink.
// If the underlying Filer implements SymLinker, it delegates to that implementation.
// Otherwise, it returns an error.
func (efs *extendedFS) Readlink(name string) (string, error) {
	name = toSlash(name)
	if !isVirtualAbs(name) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			name = path.Join(efs.cwd, name)
		}
	}
	if linker, ok := efs.filer.(symlinker); ok {
		return linker.Readlink(name)
	}
	return "", &os.PathError{Op: "readlink", Path: name, Err: errors.New("symlinks not supported")}
}

// Symlink creates a symbolic link.
// If the underlying Filer implements SymLinker, it delegates to that implementation.
// Otherwise, it returns an error.
func (efs *extendedFS) Symlink(oldname, newname string) error {
	newname = toSlash(newname)
	if !isVirtualAbs(newname) {
		if _, ok := efs.filer.(dirnavigator); !ok {
			newname = path.Join(efs.cwd, newname)
		}
	}
	if linker, ok := efs.filer.(symlinker); ok {
		return linker.Symlink(oldname, newname)
	}
	return &os.PathError{Op: "symlink", Path: newname, Err: errors.New("symlinks not supported")}
}
