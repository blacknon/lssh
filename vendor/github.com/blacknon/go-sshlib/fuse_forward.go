//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"errors"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/chroot"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fuse/nodefs"
	"github.com/hanwen/go-fuse/v2/fuse/pathfs"
)

// FUSEForward mounts a remote directory over the current SSH connection using
// an SFTP-backed FUSE filesystem. The call blocks until the mount is unmounted.
func (c *Connect) FUSEForward(mountpoint, basepoint string) (err error) {
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

	return serveFUSEMount(mountpoint, newBillyPathFS(remoteFS, "sshlib-sftp:"+basepoint))
}

// FUSEReverseForward mounts a local sharepoint via FUSE loopback and blocks
// until it is unmounted.
//
// Unlike NFSReverseForward, FUSE is host-local rather than a network service,
// so this creates the mount on the local host.
func (c *Connect) FUSEReverseForward(mountpoint, sharepoint string) (err error) {
	sharepoint, err = filepath.Abs(sharepoint)
	if err != nil {
		return err
	}

	fs := pathfs.NewLoopbackFileSystem(sharepoint)
	return serveFUSEMount(mountpoint, fs)
}

func serveFUSEMount(mountpoint string, fs pathfs.FileSystem) error {
	if mountpoint == "" {
		return errors.New("sshlib: mountpoint is required")
	}

	if err := os.MkdirAll(mountpoint, 0755); err != nil {
		return err
	}

	pfs := pathfs.NewPathNodeFs(fs, nil)
	server, _, err := nodefs.Mount(
		mountpoint,
		pfs.Root(),
		&fuse.MountOptions{
			FsName: fs.String(),
			Name:   "sshlib",
		},
		nil,
	)
	if err != nil {
		return err
	}

	if err := server.WaitMount(); err != nil {
		_ = server.Unmount()
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	go func() {
		<-sigCh
		_ = server.Unmount()
	}()

	server.Wait()
	return nil
}

type billyPathFS struct {
	pathfs.FileSystem
	fs     billy.Filesystem
	change billy.Change
	name   string
}

func newBillyPathFS(fs billy.Filesystem, name string) pathfs.FileSystem {
	pfs := &billyPathFS{
		FileSystem: pathfs.NewDefaultFileSystem(),
		fs:         fs,
		name:       name,
	}

	if change, ok := fs.(billy.Change); ok {
		pfs.change = change
	}

	return pfs
}

func (fs *billyPathFS) String() string {
	if fs.name != "" {
		return fs.name
	}

	return "sshlib-billyfs"
}

func (fs *billyPathFS) GetAttr(name string, _ *fuse.Context) (*fuse.Attr, fuse.Status) {
	info, err := fs.lstat(name)
	if err != nil {
		return nil, fuse.ToStatus(err)
	}

	attr := fileInfoToAttr(info)
	if attr == nil {
		return nil, fuse.EIO
	}

	return attr, fuse.OK
}

func (fs *billyPathFS) Chmod(name string, mode uint32, _ *fuse.Context) fuse.Status {
	if fs.change == nil {
		return fuse.ENOSYS
	}

	return fuse.ToStatus(fs.change.Chmod(fs.clean(name), os.FileMode(mode)))
}

func (fs *billyPathFS) Chown(name string, uid uint32, gid uint32, _ *fuse.Context) fuse.Status {
	if fs.change == nil {
		return fuse.ENOSYS
	}

	return fuse.ToStatus(fs.change.Chown(fs.clean(name), int(uid), int(gid)))
}

func (fs *billyPathFS) Utimens(name string, atime *time.Time, mtime *time.Time, _ *fuse.Context) fuse.Status {
	if fs.change == nil {
		return fuse.ENOSYS
	}

	now := time.Now()
	if atime == nil {
		atime = &now
	}
	if mtime == nil {
		mtime = &now
	}

	return fuse.ToStatus(fs.change.Chtimes(fs.clean(name), *atime, *mtime))
}

func (fs *billyPathFS) Truncate(name string, size uint64, _ *fuse.Context) fuse.Status {
	f, err := fs.fs.OpenFile(fs.clean(name), os.O_WRONLY, 0)
	if err != nil {
		return fuse.ToStatus(err)
	}
	defer f.Close()

	return fuse.ToStatus(f.Truncate(int64(size)))
}

func (fs *billyPathFS) Access(name string, _ uint32, _ *fuse.Context) fuse.Status {
	_, err := fs.fs.Stat(fs.clean(name))
	return fuse.ToStatus(err)
}

func (fs *billyPathFS) Link(_, _ string, _ *fuse.Context) fuse.Status {
	return fuse.ENOSYS
}

func (fs *billyPathFS) Mkdir(name string, mode uint32, _ *fuse.Context) fuse.Status {
	return fuse.ToStatus(fs.fs.MkdirAll(fs.clean(name), os.FileMode(mode)))
}

func (fs *billyPathFS) Mknod(_ string, _ uint32, _ uint32, _ *fuse.Context) fuse.Status {
	return fuse.ENOSYS
}

func (fs *billyPathFS) Rename(oldName string, newName string, _ *fuse.Context) fuse.Status {
	return fuse.ToStatus(fs.fs.Rename(fs.clean(oldName), fs.clean(newName)))
}

func (fs *billyPathFS) Rmdir(name string, _ *fuse.Context) fuse.Status {
	return fuse.ToStatus(fs.fs.Remove(fs.clean(name)))
}

func (fs *billyPathFS) Unlink(name string, _ *fuse.Context) fuse.Status {
	return fuse.ToStatus(fs.fs.Remove(fs.clean(name)))
}

func (fs *billyPathFS) Open(name string, flags uint32, _ *fuse.Context) (nodefs.File, fuse.Status) {
	file, err := fs.fs.OpenFile(fs.clean(name), int(flags)&^syscall.O_APPEND, 0)
	if err != nil {
		return nil, fuse.ToStatus(err)
	}

	return newBillyFuseFile(file), fuse.OK
}

func (fs *billyPathFS) Create(name string, flags uint32, mode uint32, _ *fuse.Context) (nodefs.File, fuse.Status) {
	file, err := fs.fs.OpenFile(fs.clean(name), int(flags)&^syscall.O_APPEND|os.O_CREATE, os.FileMode(mode))
	if err != nil {
		return nil, fuse.ToStatus(err)
	}

	return newBillyFuseFile(file), fuse.OK
}

func (fs *billyPathFS) OpenDir(name string, _ *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	infos, err := fs.fs.ReadDir(fs.clean(name))
	if err != nil {
		return nil, fuse.ToStatus(err)
	}

	out := make([]fuse.DirEntry, 0, len(infos))
	for _, info := range infos {
		entry := fuse.DirEntry{
			Name: info.Name(),
			Mode: uint32(info.Mode()),
		}

		if st := fuse.ToStatT(info); st != nil {
			entry.Mode = uint32(st.Mode)
			entry.Ino = st.Ino
		}

		out = append(out, entry)
	}

	return out, fuse.OK
}

func (fs *billyPathFS) Symlink(target string, linkName string, _ *fuse.Context) fuse.Status {
	return fuse.ToStatus(fs.fs.Symlink(target, fs.clean(linkName)))
}

func (fs *billyPathFS) Readlink(name string, _ *fuse.Context) (string, fuse.Status) {
	link, err := fs.fs.Readlink(fs.clean(name))
	if err != nil {
		return "", fuse.ToStatus(err)
	}

	return link, fuse.OK
}

func (fs *billyPathFS) clean(name string) string {
	if name == "" {
		return "."
	}

	name = filepath.Clean(name)
	if name == "." {
		return "."
	}

	return strings.TrimPrefix(name, string(filepath.Separator))
}

func (fs *billyPathFS) lstat(name string) (os.FileInfo, error) {
	if name == "" {
		return fs.fs.Stat(".")
	}

	return fs.fs.Lstat(fs.clean(name))
}

func fileInfoToAttr(info os.FileInfo) *fuse.Attr {
	if info == nil {
		return nil
	}

	if attr := fuse.ToAttr(info); attr != nil {
		return attr
	}

	mode := uint32(info.Mode().Perm())
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		mode |= syscall.S_IFLNK
	case info.IsDir():
		mode |= syscall.S_IFDIR
	default:
		mode |= syscall.S_IFREG
	}

	modTime := info.ModTime()
	return &fuse.Attr{
		Mode:  mode,
		Size:  uint64(info.Size()),
		Mtime: uint64(modTime.Unix()),
		Ctime: uint64(modTime.Unix()),
		Atime: uint64(modTime.Unix()),
	}
}

type billyFuseFile struct {
	nodefs.File
	file billy.File
	mu   sync.Mutex
}

func newBillyFuseFile(file billy.File) nodefs.File {
	return &billyFuseFile{
		File: nodefs.NewDefaultFile(),
		file: file,
	}
}

func (f *billyFuseFile) String() string {
	return "billyFuseFile(" + f.file.Name() + ")"
}

func (f *billyFuseFile) InnerFile() nodefs.File {
	return nil
}

func (f *billyFuseFile) SetInode(*nodefs.Inode) {}

func (f *billyFuseFile) Read(dest []byte, off int64) (fuse.ReadResult, fuse.Status) {
	n, err := f.file.ReadAt(dest, off)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fuse.ToStatus(err)
	}

	return fuse.ReadResultData(dest[:n]), fuse.OK
}

func (f *billyFuseFile) Write(data []byte, off int64) (uint32, fuse.Status) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if writerAt, ok := f.file.(interface {
		WriteAt([]byte, int64) (int, error)
	}); ok {
		n, err := writerAt.WriteAt(data, off)
		return uint32(n), fuse.ToStatus(err)
	}

	if _, err := f.file.Seek(off, io.SeekStart); err != nil {
		return 0, fuse.ToStatus(err)
	}

	n, err := f.file.Write(data)
	return uint32(n), fuse.ToStatus(err)
}

func (f *billyFuseFile) Flush() fuse.Status {
	if syncer, ok := f.file.(interface{ Sync() error }); ok {
		return fuse.ToStatus(syncer.Sync())
	}

	return fuse.OK
}

func (f *billyFuseFile) Release() {
	_ = f.file.Close()
}

func (f *billyFuseFile) Fsync(_ int) fuse.Status {
	return f.Flush()
}

func (f *billyFuseFile) Truncate(size uint64) fuse.Status {
	return fuse.ToStatus(f.file.Truncate(int64(size)))
}

func (f *billyFuseFile) GetAttr(out *fuse.Attr) fuse.Status {
	statter, ok := f.file.(interface {
		Stat() (os.FileInfo, error)
	})
	if !ok {
		return fuse.ENOSYS
	}

	info, err := statter.Stat()
	if err != nil {
		return fuse.ToStatus(err)
	}

	attr := fileInfoToAttr(info)
	if attr == nil {
		return fuse.EIO
	}

	*out = *attr
	return fuse.OK
}

func (f *billyFuseFile) Chown(uid uint32, gid uint32) fuse.Status {
	chowner, ok := f.file.(interface {
		Chown(int, int) error
	})
	if !ok {
		return fuse.ENOSYS
	}

	return fuse.ToStatus(chowner.Chown(int(uid), int(gid)))
}

func (f *billyFuseFile) Chmod(perms uint32) fuse.Status {
	chmoder, ok := f.file.(interface {
		Chmod(os.FileMode) error
	})
	if !ok {
		return fuse.ENOSYS
	}

	return fuse.ToStatus(chmoder.Chmod(os.FileMode(perms)))
}

func (f *billyFuseFile) Utimens(atime *time.Time, mtime *time.Time) fuse.Status {
	chtimer, ok := f.file.(interface {
		Chtimes(time.Time, time.Time) error
	})
	if !ok {
		return fuse.ENOSYS
	}

	now := time.Now()
	if atime == nil {
		atime = &now
	}
	if mtime == nil {
		mtime = &now
	}

	return fuse.ToStatus(chtimer.Chtimes(*atime, *mtime))
}
