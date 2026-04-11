// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"errors"
	"io"
	iofs "io/fs"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/absfs/absfs"
	"github.com/absfs/smbfs"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/chroot"
	osfs "github.com/go-git/go-billy/v5/osfs"
)

// SMBForward starts a local SMB server backed by a remote SFTP filesystem.
// The server runs until the process receives SIGINT or SIGTERM.
// If shareName is empty, "share" is used.
func (c *Connect) SMBForward(address, port, shareName, basepoint string) (err error) {
	client, err := c.newSFTPClient()
	if err != nil {
		return err
	}
	defer client.Close()

	homepoint, err := client.RealPath(".")
	if err != nil {
		return err
	}

	basepoint = getRemoteAbsPath(homepoint, basepoint)
	remoteFS := chroot.New(&SFTPFS{Client: client}, basepoint)

	return serveSMB(address, port, defaultSMBShareName(shareName), newAbsBillyFS(remoteFS), nil)
}

// SMBReverseForward starts a local SMB server backed by sharepoint, then exposes
// it on the remote side via SSH remote forwarding. The server runs until the
// process receives SIGINT or SIGTERM. If shareName is empty, "share" is used.
func (c *Connect) SMBReverseForward(address, port, shareName, sharepoint string) (err error) {
	sharepoint, err = filepath.Abs(sharepoint)
	if err != nil {
		return err
	}

	localFS := newAbsBillyFS(osfs.New(sharepoint))
	remoteAddr := net.JoinHostPort(address, port)

	return serveSMB("127.0.0.1", "0", defaultSMBShareName(shareName), localFS, func(localAddr string) error {
		return c.TCPRemoteForward(localAddr, remoteAddr)
	})
}

func serveSMB(host, port, shareName string, fs absfs.FileSystem, onReady func(localAddr string) error) error {
	portNum, err := net.LookupPort("tcp", port)
	if err != nil {
		return err
	}

	server, err := smbfs.NewServer(smbfs.ServerOptions{
		Hostname:        host,
		Port:            portNum,
		ServerName:      "SSHLIB",
		AllowGuest:      true,
		SigningRequired: true,
	})
	if err != nil {
		return err
	}

	if err := server.AddShare(fs, smbfs.ShareOptions{
		ShareName:  shareName,
		SharePath:  "/",
		AllowGuest: true,
	}); err != nil {
		return err
	}

	if err := server.Listen(); err != nil {
		return err
	}
	defer server.Stop()

	if onReady != nil {
		addr := server.Addr()
		if addr == nil {
			return errors.New("sshlib: SMB server has no listening address")
		}
		if err := onReady(addr.String()); err != nil {
			return err
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	<-sigCh
	return nil
}

func defaultSMBShareName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "share"
	}
	return name
}

type absBillyFS struct {
	fs  billy.Filesystem
	cwd string
}

func newAbsBillyFS(fs billy.Filesystem) absfs.FileSystem {
	return &absBillyFS{
		fs:  fs,
		cwd: "/",
	}
}

func (fsys *absBillyFS) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error) {
	file, err := fsys.fs.OpenFile(fsys.clean(name), flag, perm)
	if err != nil {
		return nil, err
	}
	return newAbsBillyFile(file), nil
}

func (fsys *absBillyFS) Mkdir(name string, perm os.FileMode) error {
	return fsys.fs.MkdirAll(fsys.clean(name), perm)
}

func (fsys *absBillyFS) Remove(name string) error {
	return fsys.fs.Remove(fsys.clean(name))
}

func (fsys *absBillyFS) Rename(oldpath, newpath string) error {
	return fsys.fs.Rename(fsys.clean(oldpath), fsys.clean(newpath))
}

func (fsys *absBillyFS) Stat(name string) (os.FileInfo, error) {
	return fsys.fs.Stat(fsys.clean(name))
}

func (fsys *absBillyFS) Chmod(name string, mode os.FileMode) error {
	if changer, ok := fsys.fs.(billy.Change); ok {
		return changer.Chmod(fsys.clean(name), mode)
	}
	return errors.New("sshlib: chmod is not supported")
}

func (fsys *absBillyFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	if changer, ok := fsys.fs.(billy.Change); ok {
		return changer.Chtimes(fsys.clean(name), atime, mtime)
	}
	return errors.New("sshlib: chtimes is not supported")
}

func (fsys *absBillyFS) Chown(name string, uid, gid int) error {
	if changer, ok := fsys.fs.(billy.Change); ok {
		return changer.Chown(fsys.clean(name), uid, gid)
	}
	return errors.New("sshlib: chown is not supported")
}

func (fsys *absBillyFS) ReadDir(name string) ([]iofs.DirEntry, error) {
	infos, err := fsys.fs.ReadDir(fsys.clean(name))
	if err != nil {
		return nil, err
	}

	entries := make([]iofs.DirEntry, 0, len(infos))
	for _, info := range infos {
		entries = append(entries, iofs.FileInfoToDirEntry(info))
	}
	return entries, nil
}

func (fsys *absBillyFS) ReadFile(name string) ([]byte, error) {
	file, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(file)
}

func (fsys *absBillyFS) Sub(dir string) (iofs.FS, error) {
	sub, err := fsys.fs.Chroot(fsys.clean(dir))
	if err != nil {
		return nil, err
	}
	return absBillySubFS{fs: &absBillyFS{fs: sub, cwd: "/"}}, nil
}

func (fsys *absBillyFS) Chdir(dir string) error {
	name := fsys.clean(dir)
	info, err := fsys.fs.Stat(name)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return &os.PathError{Op: "chdir", Path: dir, Err: errors.New("not a directory")}
	}
	fsys.cwd = absCleanPath(dir, fsys.cwd)
	return nil
}

func (fsys *absBillyFS) Getwd() (string, error) {
	return fsys.cwd, nil
}

func (fsys *absBillyFS) TempDir() string {
	return "/tmp"
}

func (fsys *absBillyFS) Open(name string) (absfs.File, error) {
	file, err := fsys.fs.Open(fsys.clean(name))
	if err != nil {
		return nil, err
	}
	return newAbsBillyFile(file), nil
}

func (fsys *absBillyFS) Create(name string) (absfs.File, error) {
	file, err := fsys.fs.Create(fsys.clean(name))
	if err != nil {
		return nil, err
	}
	return newAbsBillyFile(file), nil
}

func (fsys *absBillyFS) MkdirAll(name string, perm os.FileMode) error {
	return fsys.fs.MkdirAll(fsys.clean(name), perm)
}

func (fsys *absBillyFS) RemoveAll(name string) error {
	if remover, ok := fsys.fs.(interface{ RemoveAll(string) error }); ok {
		return remover.RemoveAll(fsys.clean(name))
	}

	return fsys.removeAll(absCleanPath(name, fsys.cwd))
}

func (fsys *absBillyFS) Truncate(name string, size int64) error {
	file, err := fsys.fs.OpenFile(fsys.clean(name), os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	return file.Truncate(size)
}

func (fsys *absBillyFS) clean(name string) string {
	return billyPath(absCleanPath(name, fsys.cwd))
}

type absBillyFile struct {
	file billy.File
}

type absBillySubFS struct {
	fs *absBillyFS
}

func (sub absBillySubFS) Open(name string) (iofs.File, error) {
	return sub.fs.Open(name)
}

func newAbsBillyFile(file billy.File) absfs.File {
	return &absBillyFile{file: file}
}

func (f *absBillyFile) Name() string                { return f.file.Name() }
func (f *absBillyFile) Read(p []byte) (int, error)  { return f.file.Read(p) }
func (f *absBillyFile) Write(p []byte) (int, error) { return f.file.Write(p) }
func (f *absBillyFile) Close() error                { return f.file.Close() }

func (f *absBillyFile) Sync() error {
	if syncer, ok := f.file.(interface{ Sync() error }); ok {
		return syncer.Sync()
	}
	return nil
}

func (f *absBillyFile) Stat() (os.FileInfo, error) {
	if statter, ok := f.file.(interface{ Stat() (os.FileInfo, error) }); ok {
		return statter.Stat()
	}
	return nil, errors.New("sshlib: stat is not supported")
}

func (f *absBillyFile) Readdir(n int) ([]os.FileInfo, error) {
	if reader, ok := f.file.(interface {
		Readdir(int) ([]os.FileInfo, error)
	}); ok {
		return reader.Readdir(n)
	}
	return nil, errors.New("sshlib: readdir is not supported")
}

func (f *absBillyFile) Seek(offset int64, whence int) (int64, error) {
	return f.file.Seek(offset, whence)
}
func (f *absBillyFile) ReadAt(p []byte, off int64) (int, error) { return f.file.ReadAt(p, off) }

func (f *absBillyFile) WriteAt(p []byte, off int64) (int, error) {
	if writerAt, ok := f.file.(interface {
		WriteAt([]byte, int64) (int, error)
	}); ok {
		return writerAt.WriteAt(p, off)
	}

	if _, err := f.file.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	return f.file.Write(p)
}

func (f *absBillyFile) WriteString(s string) (int, error) { return io.WriteString(f.file, s) }
func (f *absBillyFile) Truncate(size int64) error         { return f.file.Truncate(size) }

func (f *absBillyFile) Readdirnames(n int) ([]string, error) {
	infos, err := f.Readdir(n)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(infos))
	for _, info := range infos {
		names = append(names, info.Name())
	}
	return names, nil
}

func (f *absBillyFile) ReadDir(n int) ([]iofs.DirEntry, error) {
	if reader, ok := f.file.(interface {
		ReadDir(int) ([]iofs.DirEntry, error)
	}); ok {
		return reader.ReadDir(n)
	}

	infos, err := f.Readdir(n)
	if err != nil {
		return nil, err
	}

	entries := make([]iofs.DirEntry, 0, len(infos))
	for _, info := range infos {
		entries = append(entries, iofs.FileInfoToDirEntry(info))
	}
	return entries, nil
}

func (fsys *absBillyFS) removeAll(name string) error {
	info, err := fsys.Stat(name)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fsys.Remove(name)
	}

	entries, err := fsys.ReadDir(name)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		child := pathJoin(name, entry.Name())
		if err := fsys.removeAll(child); err != nil {
			return err
		}
	}

	return fsys.Remove(name)
}

func absCleanPath(name, cwd string) string {
	name = filepath.ToSlash(name)
	if strings.TrimSpace(name) == "" {
		name = "."
	}

	if !strings.HasPrefix(name, "/") {
		name = pathJoin(cwd, name)
	}

	return pathClean(name)
}

func billyPath(abs string) string {
	cleaned := strings.TrimPrefix(pathClean(abs), "/")
	if cleaned == "" {
		return "."
	}
	return cleaned
}

func pathClean(p string) string {
	parts := strings.Split(strings.ReplaceAll(p, "\\", "/"), "/")
	stack := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		default:
			stack = append(stack, part)
		}
	}
	return "/" + strings.Join(stack, "/")
}

func pathJoin(elem ...string) string {
	return pathClean(strings.Join(elem, "/"))
}
