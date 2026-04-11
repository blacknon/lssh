package smbfs

import (
	"io/fs"
	"time"
)

// SMBSession abstracts an SMB session for testability.
// This interface wraps the go-smb2 Session type.
type SMBSession interface {
	// Mount mounts a share and returns an SMBShare interface.
	Mount(shareName string) (SMBShare, error)
	// Logoff ends the session.
	Logoff() error
}

// SMBShare abstracts an SMB share for testability.
// This interface wraps the go-smb2 Share type.
type SMBShare interface {
	// OpenFile opens a file with the specified flags and permissions.
	OpenFile(name string, flag int, perm fs.FileMode) (SMBFile, error)
	// Stat returns file info for the specified path.
	Stat(name string) (fs.FileInfo, error)
	// Mkdir creates a directory.
	Mkdir(name string, perm fs.FileMode) error
	// Remove removes a file or empty directory.
	Remove(name string) error
	// Rename renames a file or directory.
	Rename(oldname, newname string) error
	// Chmod changes the mode of a file.
	Chmod(name string, mode fs.FileMode) error
	// Chtimes changes the access and modification times of a file.
	Chtimes(name string, atime, mtime time.Time) error
	// Umount unmounts the share.
	Umount() error
}

// SMBFile abstracts an SMB file handle for testability.
// This interface wraps the go-smb2 File type.
type SMBFile interface {
	// Read reads up to len(p) bytes into p.
	Read(p []byte) (n int, err error)
	// Write writes len(p) bytes from p to the file.
	Write(p []byte) (n int, err error)
	// Seek sets the offset for the next Read or Write.
	Seek(offset int64, whence int) (int64, error)
	// Close closes the file.
	Close() error
	// Stat returns file information.
	Stat() (fs.FileInfo, error)
	// Readdir reads the directory contents.
	Readdir(n int) ([]fs.FileInfo, error)
}

// SMBDialer abstracts the SMB connection dialer for testability.
// This allows injection of mock dialers for testing.
type SMBDialer interface {
	// Dial establishes an SMB session over the given connection.
	// The conn parameter should be a net.Conn.
	Dial(conn interface{}) (SMBSession, error)
}

// ConnectionFactory creates SMB connections for the connection pool.
// This abstraction allows injection of mock connections for testing.
type ConnectionFactory interface {
	// CreateConnection creates a new SMB connection using the provided config.
	CreateConnection(config *Config) (SMBSession, SMBShare, error)
}
