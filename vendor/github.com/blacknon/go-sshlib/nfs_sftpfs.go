// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/chroot"
	"github.com/go-git/go-billy/v5/helper/temporal"
	"github.com/pkg/sftp"
)

// sftpFile is a wrapper around sftp.File that implements the billy.File interface
type sftpFile struct {
	*sftp.File
	mx sync.Mutex
}

func (f *sftpFile) Lock() error {
	f.mx.Lock()
	return nil
}

func (f *sftpFile) Unlock() error {
	f.mx.Unlock()
	return nil
}

func NewChangeSFTPFS(client *sftp.Client, base string) billy.Filesystem {
	baseFS := &SFTPFS{Client: client}
	rooted := temporal.New(chroot.New(baseFS, base), "")
	return &changeChrootFS{
		Filesystem: rooted,
		root:       filepath.Clean(base),
		change:     baseFS,
	}
}

type changeChrootFS struct {
	billy.Filesystem
	root   string
	change billy.Change
}

var _ billy.Change = (*changeChrootFS)(nil)

func (fs *changeChrootFS) changePath(name string) (string, error) {
	if isCrossBoundaryPath(name) {
		return "", billy.ErrCrossedBoundary
	}

	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == string(filepath.Separator) {
		return fs.root, nil
	}

	clean = strings.TrimPrefix(clean, string(filepath.Separator))
	return filepath.Join(fs.root, clean), nil
}

func (fs *changeChrootFS) Chmod(name string, mode os.FileMode) error {
	fullpath, err := fs.changePath(name)
	if err != nil {
		return err
	}
	return fs.change.Chmod(fullpath, mode)
}

func (fs *changeChrootFS) Lchown(name string, uid, gid int) error {
	fullpath, err := fs.changePath(name)
	if err != nil {
		return err
	}
	return fs.change.Lchown(fullpath, uid, gid)
}

func (fs *changeChrootFS) Chown(name string, uid, gid int) error {
	fullpath, err := fs.changePath(name)
	if err != nil {
		return err
	}
	return fs.change.Chown(fullpath, uid, gid)
}

func (fs *changeChrootFS) Chtimes(name string, atime, mtime time.Time) error {
	fullpath, err := fs.changePath(name)
	if err != nil {
		return err
	}
	return fs.change.Chtimes(fullpath, atime, mtime)
}

func isCrossBoundaryPath(path string) bool {
	path = filepath.ToSlash(path)
	path = filepath.Clean(path)
	return strings.HasPrefix(path, ".."+string(filepath.Separator))
}

type SFTPFS struct {
	billy.Filesystem
	Client *sftp.Client
	mu     sync.Mutex
}

var _ billy.Change = (*SFTPFS)(nil)

// Create
func (fs *SFTPFS) Create(filename string) (billy.File, error) {
	return fs.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
}

// OpenFile
func (fs *SFTPFS) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	return fs.openFile(filename, flag, perm, fs.createDir)
}

func (fs *SFTPFS) openFile(fn string, flag int, perm os.FileMode, createDir func(string) error) (billy.File, error) {
	if flag&os.O_CREATE != 0 {
		if createDir == nil {
			return nil, fmt.Errorf("createDir func cannot be nil if file needs to be opened in create mode")
		}
		if err := createDir(fn); err != nil {
			return nil, err
		}
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	f, err := fs.Client.OpenFile(fn, flag)
	if err != nil {
		return nil, err
	}

	return &sftpFile{File: f}, nil
}

func (fs *SFTPFS) createDir(fullpath string) error {
	dir := filepath.Dir(fullpath)
	if dir != "." {
		fs.mu.Lock()
		defer fs.mu.Unlock()
		if err := fs.Client.MkdirAll(dir); err != nil {
			return err
		}
	}

	return nil
}

// ReadDir
func (fs *SFTPFS) ReadDir(path string) ([]os.FileInfo, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	l, err := fs.Client.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var s = make([]os.FileInfo, len(l))
	for i, f := range l {
		s[i] = f
	}

	return s, nil
}

// Rename
func (fs *SFTPFS) Rename(from, to string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.Client.Rename(from, to)
}

// MkdirAll
func (fs *SFTPFS) MkdirAll(filename string, perm os.FileMode) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	err := fs.Client.MkdirAll(filename)
	if err != nil {
		return err
	}

	return nil
}

// Open
func (fs *SFTPFS) Open(filename string) (billy.File, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	f, err := fs.Client.Open(filename)
	if err != nil {
		return nil, err
	}
	return &sftpFile{File: f}, nil
}

// Stat
func (fs *SFTPFS) Stat(filename string) (os.FileInfo, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.Client.Stat(filepath.Clean(filename))
}

// Remove
func (fs *SFTPFS) Remove(filename string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.Client.Remove(filename)
}

// TempFile
func (fs *SFTPFS) TempFile(dir, prefix string) (billy.File, error) {
	if err := fs.createDir(dir + string(os.PathSeparator)); err != nil {
		return nil, err
	}

	tempFileName := prefix + strconv.FormatInt(time.Now().UnixNano(), 10)
	tempFilePath := filepath.Join(dir, tempFileName)

	f, err := fs.Create(tempFilePath)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func (fs *SFTPFS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// RemoveAll
func (fs *SFTPFS) RemoveAll(filename string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.Client.RemoveAll(filename)
}

// Lstat
func (fs *SFTPFS) Lstat(filename string) (os.FileInfo, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.Client.Lstat(filepath.Clean(filename))
}

// Symlink
func (fs *SFTPFS) Symlink(target, link string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.Client.Symlink(target, link)
}

// Readlink
func (fs *SFTPFS) Readlink(link string) (string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.Client.ReadLink(link)
}

func (fs *SFTPFS) Chmod(name string, mode os.FileMode) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.Client.Chmod(name, mode)
}

func (fs *SFTPFS) Lchown(name string, uid, gid int) error {
	return fs.Chown(name, uid, gid)
}

func (fs *SFTPFS) Chown(name string, uid, gid int) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.Client.Chown(name, uid, gid)
}

func (fs *SFTPFS) Chtimes(name string, atime, mtime time.Time) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.Client.Chtimes(name, atime, mtime)
}

// Capabilities
func (fs *SFTPFS) Capabilities() billy.Capability {
	return billy.DefaultCapabilities
}
