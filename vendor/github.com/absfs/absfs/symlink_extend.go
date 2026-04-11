package absfs

import (
	"os"
	"syscall"
)

// symlinkFS combines a FileSystem and SymLinker into a SymlinkFileSystem.
// Used when the underlying Filer has native symlink support.
type symlinkFS struct {
	FileSystem
	SymLinker
}

// Ensure symlinkFS implements SymlinkFileSystem
var _ SymlinkFileSystem = (*symlinkFS)(nil)

// ExtendSymlinkFiler returns a SymlinkFileSystem from any Filer.
//
// This function intelligently handles different capabilities:
//   - If filer already implements SymlinkFileSystem, returns it directly
//   - If filer implements both FileSystem and SymLinker, combines them
//   - If filer implements SymLinker but not FileSystem, extends and combines
//   - If filer has no symlink support, wraps with SymlinkOverlay
//
// This is the recommended way to get a SymlinkFileSystem from any Filer.
// Calling it multiple times is safe - it won't double-wrap.
//
// Example:
//
//	filer := myFS{}                            // Implement Filer (9 methods)
//	fs := absfs.ExtendFiler(filer)             // Get FileSystem
//	sfs := absfs.ExtendSymlinkFiler(filer)    // Get SymlinkFileSystem
func ExtendSymlinkFiler(filer Filer) SymlinkFileSystem {
	// Already a SymlinkFileSystem? Return as-is
	if sfs, ok := filer.(SymlinkFileSystem); ok {
		return sfs
	}

	// Has both FileSystem and SymLinker? Combine them
	if fs, ok := filer.(FileSystem); ok {
		if sl, ok := filer.(SymLinker); ok {
			return &symlinkFS{FileSystem: fs, SymLinker: sl}
		}
	}

	// Has SymLinker but not FileSystem? Extend and combine
	if sl, ok := filer.(SymLinker); ok {
		return &symlinkFS{
			FileSystem: ExtendFiler(filer),
			SymLinker:  sl,
		}
	}

	// No native symlink support - add overlay
	return NewSymlinkOverlay(ExtendFiler(filer))
}

// NoSymlinks wraps a FileSystem and actively blocks all symlink operations.
// Use this when your application must not allow symlinks under any circumstances.
//
// Operations blocked:
//   - Symlink: returns ENOTSUP
//   - Readlink: returns ENOTSUP
//   - Lstat: falls through to Stat (always follows symlinks)
//   - Lchown: falls through to Chown (always follows symlinks)
//
// Note: This doesn't prevent symlinks from existing if the underlying filesystem
// has them. It only prevents creation of new symlinks and reading existing ones
// through this wrapper.
type NoSymlinks struct {
	FileSystem
}

// Ensure NoSymlinks implements SymLinker (with blocking behavior)
var _ SymLinker = (*NoSymlinks)(nil)

// Symlink always returns ENOTSUP
func (n *NoSymlinks) Symlink(oldname, newname string) error {
	return &os.PathError{Op: "symlink", Path: newname, Err: syscall.ENOTSUP}
}

// Readlink always returns ENOTSUP
func (n *NoSymlinks) Readlink(name string) (string, error) {
	return "", &os.PathError{Op: "readlink", Path: name, Err: syscall.ENOTSUP}
}

// Lstat follows symlinks (same as Stat) - symlinks are invisible
func (n *NoSymlinks) Lstat(name string) (os.FileInfo, error) {
	return n.FileSystem.Stat(name)
}

// Lchown follows symlinks (same as Chown) - symlinks are invisible
func (n *NoSymlinks) Lchown(name string, uid, gid int) error {
	return n.FileSystem.Chown(name, uid, gid)
}

// BlockSymlinks wraps a FileSystem to block all symlink operations.
// This is a convenience function that returns a SymlinkFileSystem
// where all symlink operations fail with ENOTSUP.
func BlockSymlinks(fs FileSystem) SymlinkFileSystem {
	return &symlinkFS{
		FileSystem: fs,
		SymLinker:  &NoSymlinks{FileSystem: fs},
	}
}
