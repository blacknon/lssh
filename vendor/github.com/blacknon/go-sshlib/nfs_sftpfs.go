// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"os"
	"path/filepath"
	"strconv"
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
	return temporal.New(
		chroot.New(&SFTPFS{Client: client}, base),
		"",
	)
}

type SFTPFS struct {
	billy.Filesystem
	Client *sftp.Client
}

// Create
func (fs *SFTPFS) Create(filename string) (billy.File, error) {
	_, err := fs.Stat(filename)
	if err == nil {
		return nil, os.ErrExist
	}

	dir := filepath.Dir(filename)
	err = fs.MkdirAll(dir, os.ModeDir)
	if err != nil {
		return nil, err
	}

	f, err := fs.Client.Create(filename)
	if err != nil {
		return nil, err
	}
	return &sftpFile{File: f}, nil
}

// OpenFile
func (fs *SFTPFS) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	// TODO: create dirをする
	// https://github.com/src-d/go-billy/blob/master/osfs/os.go#L31-L54

	f, err := fs.Client.OpenFile(filename, flag)
	if err != nil {
		return nil, err
	}

	return &sftpFile{File: f}, nil
}

func (fs *SFTPFS) createDir(fullpath string) error {
	dir := filepath.Dir(fullpath)
	if dir != "." {
		if err := fs.Client.MkdirAll(dir); err != nil {
			return err
		}
	}

	return nil
}

// ReadDir
func (fs *SFTPFS) ReadDir(path string) ([]os.FileInfo, error) {
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
	return fs.Client.Rename(from, to)
}

// MkdirAll
func (fs *SFTPFS) MkdirAll(filename string, perm os.FileMode) error {
	err := fs.Client.MkdirAll(filename)
	if err != nil {
		return err
	}

	return nil
}

// Open
func (fs *SFTPFS) Open(filename string) (billy.File, error) {
	f, err := fs.Client.Open(filename)
	if err != nil {
		return nil, err
	}
	return &sftpFile{File: f}, nil
}

// Stat
func (fs *SFTPFS) Stat(filename string) (os.FileInfo, error) {
	return fs.Client.Stat(filepath.Clean(filename))
}

// Remove
func (fs *SFTPFS) Remove(filename string) error {
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
	return fs.Client.RemoveAll(filename)
}

// Lstat
func (fs *SFTPFS) Lstat(filename string) (os.FileInfo, error) {
	return fs.Client.Lstat(filepath.Clean(filename))
}

// Symlink
func (fs *SFTPFS) Symlink(target, link string) error {
	return fs.Client.Symlink(target, link)
}

// Readlink
func (fs *SFTPFS) Readlink(link string) (string, error) {
	return fs.Client.ReadLink(link)
}

// Capabilities
func (fs *SFTPFS) Capabilities() billy.Capability {
	return billy.DefaultCapabilities
}
