package smbfs

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"time"

	"github.com/hirochachacha/go-smb2"
)

// realSMBSession wraps a go-smb2 Session to implement SMBSession.
type realSMBSession struct {
	session *smb2.Session
}

// Mount mounts a share and returns an SMBShare interface.
func (s *realSMBSession) Mount(shareName string) (SMBShare, error) {
	share, err := s.session.Mount(shareName)
	if err != nil {
		return nil, err
	}
	return &realSMBShare{share: share}, nil
}

// Logoff ends the session.
func (s *realSMBSession) Logoff() error {
	return s.session.Logoff()
}

// realSMBShare wraps a go-smb2 Share to implement SMBShare.
type realSMBShare struct {
	share *smb2.Share
}

// OpenFile opens a file with the specified flags and permissions.
func (sh *realSMBShare) OpenFile(name string, flag int, perm fs.FileMode) (SMBFile, error) {
	file, err := sh.share.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return &realSMBFile{file: file}, nil
}

// Stat returns file info for the specified path.
func (sh *realSMBShare) Stat(name string) (fs.FileInfo, error) {
	return sh.share.Stat(name)
}

// Mkdir creates a directory.
func (sh *realSMBShare) Mkdir(name string, perm fs.FileMode) error {
	return sh.share.Mkdir(name, perm)
}

// Remove removes a file or empty directory.
func (sh *realSMBShare) Remove(name string) error {
	return sh.share.Remove(name)
}

// Rename renames a file or directory.
func (sh *realSMBShare) Rename(oldname, newname string) error {
	return sh.share.Rename(oldname, newname)
}

// Chmod changes the mode of a file.
func (sh *realSMBShare) Chmod(name string, mode fs.FileMode) error {
	return sh.share.Chmod(name, mode)
}

// Chtimes changes the access and modification times of a file.
func (sh *realSMBShare) Chtimes(name string, atime, mtime time.Time) error {
	return sh.share.Chtimes(name, atime, mtime)
}

// Umount unmounts the share.
func (sh *realSMBShare) Umount() error {
	return sh.share.Umount()
}

// realSMBFile wraps a go-smb2 File to implement SMBFile.
type realSMBFile struct {
	file *smb2.File
}

// Read reads up to len(p) bytes into p.
func (f *realSMBFile) Read(p []byte) (n int, err error) {
	return f.file.Read(p)
}

// Write writes len(p) bytes from p to the file.
func (f *realSMBFile) Write(p []byte) (n int, err error) {
	return f.file.Write(p)
}

// Seek sets the offset for the next Read or Write.
func (f *realSMBFile) Seek(offset int64, whence int) (int64, error) {
	return f.file.Seek(offset, whence)
}

// Close closes the file.
func (f *realSMBFile) Close() error {
	return f.file.Close()
}

// Stat returns file information.
func (f *realSMBFile) Stat() (fs.FileInfo, error) {
	return f.file.Stat()
}

// Readdir reads the directory contents.
func (f *realSMBFile) Readdir(n int) ([]fs.FileInfo, error) {
	return f.file.Readdir(n)
}

// RealConnectionFactory implements ConnectionFactory using real SMB connections.
type RealConnectionFactory struct{}

// CreateConnection creates a real SMB connection.
func (f *RealConnectionFactory) CreateConnection(config *Config) (SMBSession, SMBShare, error) {
	addr := fmt.Sprintf("%s:%d", config.Server, config.Port)

	// Create TCP connection with timeout
	dialer := &net.Dialer{
		Timeout: config.ConnTimeout,
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.ConnTimeout)
	defer cancel()

	netConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	// Create SMB session
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     config.Username,
			Password: config.Password,
			Domain:   config.Domain,
		},
	}

	session, err := d.Dial(netConn)
	if err != nil {
		netConn.Close()
		return nil, nil, fmt.Errorf("SMB session setup failed: %w", err)
	}

	// Connect to share
	share, err := session.Mount(config.Share)
	if err != nil {
		_ = session.Logoff()
		netConn.Close()
		return nil, nil, fmt.Errorf("failed to mount share %s: %w", config.Share, err)
	}

	return &realSMBSession{session: session}, &realSMBShare{share: share}, nil
}
