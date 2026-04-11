package absfs

import (
	"io"
	"io/fs"
	"os"
)

// UnSeekable - is an interface for file handles that can perform reads and/or
// writes but cannot perform seek operations. An UnSeekable is also
// an io.ReadWriteCloser.
type UnSeekable interface {

	// Name returns the name of the file as presented to Open.
	Name() string

	// Read reads up to len(b) bytes from the File. It returns the number of bytes
	// read and any error encountered. At end of file, Read returns 0, io.EOF.
	Read(b []byte) (int, error)

	// Write writes len(b) bytes to the File. It returns the number of bytes
	// written and an error, if any. Write returns a non-nil error when
	// n != len(b).
	Write(b []byte) (int, error)

	// Close - closes the File, rendering it unusable for I/O. It returns an error,
	// if any.
	Close() error

	// Sync - commits the current contents of the file to stable storage.
	Sync() error

	// Stat - returns the FileInfo structure describing file. If there is an
	// error, it should be of type `*os.PathError`.
	Stat() (os.FileInfo, error)

	// Readdir - reads the contents of the directory associated with file and
	// returns a slice of up to n `os.FileInfo` values, as would be returned by
	// Stat or Lstat in directory order. Subsequent calls on the same file will
	// yield further `os.FileInfos`.
	//
	// If n > 0, Readdir returns at most n FileInfo structures. In this case, if
	// Readdir returns an empty slice, it will return a non-nil error explaining
	// why. At the end of a directory, the error is io.EOF.
	//
	// If n <= 0, Readdir returns all the FileInfo from the directory in a single
	// slice. In this case, if Readdir succeeds (reads all the way to the end of
	// the directory), it returns the slice and a nil error. If it encounters an
	// error before the end of the directory, Readdir returns the FileInfo read
	// until that point and a non-nil error.
	Readdir(int) ([]os.FileInfo, error)
}

// Seekable - is an interface for file handles that can perform reads and/or
// writes and can seek to specific locations within a file. A Seekable is also
// an io.ReadWriteCloser, and an io.Seeker.
type Seekable interface {
	UnSeekable
	io.Seeker
}

// File - is an interface for file handles that supports the most common
// file operations. Filesystem interfaces in this package have functions
// that return values of type `File`, but be aware that some implementations may
// only support a subset of these functions given the limitations particular
// file systems.
type File interface {
	Seekable

	// ReadAt reads len(b) bytes from the File starting at byte offset off. It
	// returns the number of bytes read and the error, if any. ReadAt always
	// returns a non-nil error when n < len(b). At end of file, that error is
	// io.EOF.
	ReadAt(b []byte, off int64) (n int, err error)

	// WriteAt writes len(b) bytes to the File starting at byte offset off. It
	// returns the number of bytes written and an error, if any. WriteAt returns
	// a non-nil error when n != len(b).
	WriteAt(b []byte, off int64) (n int, err error)

	// WriteString is like Write, but writes the contents of string s rather than
	// a slice of bytes.
	WriteString(s string) (n int, err error)

	// Truncate - changes the size of the file. It does not change the I/O offset.
	// If there is an error, it should be of type `*os.PathError`.
	Truncate(size int64) error

	// Readdirnames - reads the contents of the directory associated with file and
	// returns a slice of up to n names. Subsequent calls on the same
	// file will yield further names.
	//
	// If n > 0, Readdirnames returns at most n names. In this case, if
	// Readdirnames returns an empty slice, it will return a non-nil error
	// explaining why. At the end of a directory, the error is io.EOF.
	//
	// If n <= 0, Readdirnames returns all the names from the directory in a single
	// slice. In this case, if Readdirnames succeeds (reads all the way to the end of
	// the directory), it returns the slice and a nil error. If it encounters an
	// error before the end of the directory, Readdirnames returns the names read
	// until that point and a non-nil error.
	Readdirnames(n int) (names []string, err error)

	// ReadDir reads the contents of the directory and returns a slice of up to n
	// DirEntry values in directory order. This is the modern Go 1.16+ equivalent
	// of Readdir that returns lightweight DirEntry values instead of full FileInfo.
	//
	// If n > 0, ReadDir returns at most n entries. In this case, if ReadDir
	// returns an empty slice, it will return a non-nil error explaining why.
	// At the end of a directory, the error is io.EOF.
	//
	// If n <= 0, ReadDir returns all entries from the directory in a single slice.
	// In this case, if ReadDir succeeds (reads all the way to the end of the
	// directory), it returns the slice and a nil error.
	ReadDir(n int) ([]fs.DirEntry, error)
}

// ExtendSeekable - extends a `Seekable` interface implementation to a File
// interface implementation. First type assertion is used to check if the
// argument already supports the `File` interface.
//
// If the argument's native type is not recognized, and cannot be type asserted
// to an implementation of `File` `ExtendSeekable` wraps the argument in a type
// that implements the additional `File` functions using only the functions
// in the `Seekable` interface. Functions that are identical between `File` and
// `Seekable` are passed through unaltered. This should would fine, but may have
// have performance implications.
func ExtendSeekable(sf Seekable) File {
	if f, ok := sf.(File); ok {
		return f
	}

	return &fileadapter{sf}
}
