package absfs

import (
	"io/fs"
	"os"
	"syscall"
)

// InvalidFile is a no-op implementation of File that should be returned from
// any file open methods when an error occurs. InvalidFile mimics the behavior
// of file handles returnd by the `os` package when there is an error.
type InvalidFile struct {

	// Path should be set to the file path provided to the function opening the
	// file (i.e. OpenFile, Open, Create).
	Path string
}

// Name - returns the name of the file path provided to the function that
// attempted to open the file.
func (f *InvalidFile) Name() string {
	return f.Path
}

// Read - returns an *os.PathError indicating a bad file handle.
func (f *InvalidFile) Read(p []byte) (int, error) {
	return 0, &os.PathError{Op: "read", Path: f.Name(), Err: syscall.EBADF}
}

// Write - returns an *os.PathError indicating a bad file handle.
func (f *InvalidFile) Write(p []byte) (int, error) {
	return 0, &os.PathError{Op: "write", Path: f.Name(), Err: syscall.EBADF}
}

// Close - does nothing and does not return an error.
func (f *InvalidFile) Close() error {
	return nil
}

// Sync - returns an *os.PathError indicating a bad file handle.
func (f *InvalidFile) Sync() error {
	return &os.PathError{Op: "sync", Path: f.Name(), Err: syscall.EBADF}
}

// Stat - returns an *os.PathError indicating a bad file handle.
func (f *InvalidFile) Stat() (os.FileInfo, error) {
	return nil, &os.PathError{Op: "stat", Path: f.Name(), Err: syscall.EBADF}
}

// Readdir - returns an *os.PathError indicating a bad file handle.
func (f *InvalidFile) Readdir(int) ([]os.FileInfo, error) {
	return nil, &os.PathError{Op: "readdir", Path: f.Name(), Err: syscall.EBADF}
}

// Seek - returns an *os.PathError indicating a bad file handle.
func (f *InvalidFile) Seek(offset int64, whence int) (ret int64, err error) {
	return 0, &os.PathError{Op: "seek", Path: f.Name(), Err: syscall.EBADF}
}

// ReadAt - returns an *os.PathError indicating a bad file handle.
func (f *InvalidFile) ReadAt(b []byte, off int64) (n int, err error) {
	return 0, &os.PathError{Op: "read", Path: f.Name(), Err: syscall.EBADF}
}

// WriteAt - returns an *os.PathError indicating a bad file handle.
func (f *InvalidFile) WriteAt(b []byte, off int64) (n int, err error) {
	return 0, &os.PathError{Op: "write", Path: f.Name(), Err: syscall.EBADF}
}

// WriteString - returns an *os.PathError indicating a bad file handle.
func (f *InvalidFile) WriteString(s string) (n int, err error) {
	return 0, &os.PathError{Op: "write", Path: f.Name(), Err: syscall.EBADF}
}

// Truncate - returns an *os.PathError indicating a bad file handle.
func (f *InvalidFile) Truncate(size int64) error {
	return &os.PathError{Op: "truncate", Path: f.Name(), Err: syscall.EBADF}
}

// Readdirnames - returns an *os.PathError indicating a bad file handle.
func (f *InvalidFile) Readdirnames(n int) (names []string, err error) {
	return nil, &os.PathError{Op: "readdirnames", Path: f.Name(), Err: syscall.EBADF}
}

// ReadDir - returns an *os.PathError indicating a bad file handle.
func (f *InvalidFile) ReadDir(n int) ([]fs.DirEntry, error) {
	return nil, &os.PathError{Op: "readdir", Path: f.Name(), Err: syscall.EBADF}
}
