//go:build linux || darwin

package lsshfs

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	pathpkg "path"
	"syscall"
	"time"

	lsshssh "github.com/blacknon/lssh/internal/ssh"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/pkg/sftp"
)

type sftpMountConn struct {
	handle    *lsshssh.SFTPClientHandle
	readWrite bool
	server    *fuse.Server
}

func newSFTPMountConn(handle *lsshssh.SFTPClientHandle, readWrite bool) (*sftpMountConn, error) {
	if handle == nil || handle.Client == nil {
		return nil, fmt.Errorf("sftp client is not available")
	}
	return &sftpMountConn{handle: handle, readWrite: readWrite}, nil
}

func (c *sftpMountConn) Close() error {
	if c.server != nil {
		_ = c.server.Unmount()
		c.server = nil
	}
	if c.handle != nil && c.handle.Closer != nil {
		return c.handle.Closer.Close()
	}
	return nil
}

func (c *sftpMountConn) CheckClientAlive() error {
	if c.handle == nil || c.handle.Client == nil {
		return fmt.Errorf("sftp client is not available")
	}
	if c.handle.SSHConnect != nil {
		if err := c.handle.SSHConnect.CheckClientAlive(); err != nil {
			return err
		}
	}
	_, err := c.handle.Client.Stat(".")
	return err
}

func (c *sftpMountConn) FUSEForward(local, remote string) error {
	root := newSFTPRoot(c.handle.Client, remote, c.readWrite)
	opts := &fs.Options{
		MountOptions: fuse.MountOptions{
			FsName: "lsshfs-sftp:" + remote,
			Name:   "lsshfs-sftp",
		},
		NullPermissions: true,
	}
	if !c.readWrite {
		opts.MountOptions.Options = append(opts.MountOptions.Options, "ro")
	}

	server, err := fs.Mount(local, root, opts)
	if err != nil {
		return err
	}
	c.server = server
	server.Wait()
	return nil
}

func (c *sftpMountConn) NFSForward(bindAddr, port, remote string) error {
	return fmt.Errorf("nfs forward is not supported for connector-backed lsshfs")
}

func (c *sftpMountConn) SMBForward(bindAddr, port, shareName, remote string) error {
	return fmt.Errorf("smb forward is not supported for connector-backed lsshfs")
}

type sftpNode struct {
	fs.Inode
	client    *sftp.Client
	basePath  string
	relPath   string
	readWrite bool
}

func newSFTPRoot(client *sftp.Client, remote string, readWrite bool) *sftpNode {
	return &sftpNode{
		client:    client,
		basePath:  cleanRemotePath(remote),
		relPath:   "",
		readWrite: readWrite,
	}
}

func (n *sftpNode) fullPath() string {
	if n.relPath == "" {
		return n.basePath
	}
	return pathpkg.Join(n.basePath, n.relPath)
}

func (n *sftpNode) childPath(name string) string {
	if n.relPath == "" {
		return name
	}
	return pathpkg.Join(n.relPath, name)
}

func (n *sftpNode) childNode(name string) *sftpNode {
	return &sftpNode{
		client:    n.client,
		basePath:  n.basePath,
		relPath:   n.childPath(name),
		readWrite: n.readWrite,
	}
}

func (n *sftpNode) stableAttr(fi os.FileInfo) fs.StableAttr {
	return fs.StableAttr{
		Mode: uint32(fi.Mode()),
		Ino:  hashPath(n.fullPath()),
	}
}

func (n *sftpNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	fi, err := n.client.Lstat(n.fullPath())
	if err != nil {
		return fs.ToErrno(err)
	}
	fillAttrFromFileInfo(out, fi, hashPath(n.fullPath()))
	out.SetTimeout(time.Second)
	return fs.OK
}

func (n *sftpNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	child := n.childNode(name)
	fi, err := n.client.Lstat(child.fullPath())
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	fillEntryFromFileInfo(out, fi, hashPath(child.fullPath()))
	ch := n.NewInode(ctx, child, child.stableAttr(fi))
	return ch, fs.OK
}

func (n *sftpNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	entries, err := n.client.ReadDir(n.fullPath())
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	result := make([]fuse.DirEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, fuse.DirEntry{
			Name: entry.Name(),
			Mode: entryMode(entry.Mode()),
			Ino:  hashPath(pathpkg.Join(n.fullPath(), entry.Name())),
		})
	}
	return fs.NewListDirStream(result), fs.OK
}

func (n *sftpNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	if !n.readWrite && isWriteOpen(flags) {
		return nil, 0, syscall.EROFS
	}

	file, err := n.client.OpenFile(n.fullPath(), int(flags))
	if err != nil {
		return nil, 0, fs.ToErrno(err)
	}

	fuseFlags := uint32(0)
	if !n.readWrite {
		fuseFlags = fuse.FOPEN_KEEP_CACHE
	}
	return &sftpFileHandle{file: file, client: n.client, path: n.fullPath()}, fuseFlags, fs.OK
}

func (n *sftpNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	if !n.readWrite {
		return nil, nil, 0, syscall.EROFS
	}

	child := n.childNode(name)
	file, err := n.client.OpenFile(child.fullPath(), int(flags)|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}
	if err := n.client.Chmod(child.fullPath(), os.FileMode(mode)); err != nil {
		_ = file.Close()
		return nil, nil, 0, fs.ToErrno(err)
	}

	fi, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, 0, fs.ToErrno(err)
	}
	fillEntryFromFileInfo(out, fi, hashPath(child.fullPath()))
	ch := n.NewInode(ctx, child, child.stableAttr(fi))
	return ch, &sftpFileHandle{file: file, client: n.client, path: child.fullPath()}, 0, fs.OK
}

func (n *sftpNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if !n.readWrite {
		return nil, syscall.EROFS
	}

	child := n.childNode(name)
	if err := n.client.Mkdir(child.fullPath()); err != nil {
		return nil, fs.ToErrno(err)
	}
	if err := n.client.Chmod(child.fullPath(), os.FileMode(mode)); err != nil {
		return nil, fs.ToErrno(err)
	}
	fi, err := n.client.Lstat(child.fullPath())
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	fillEntryFromFileInfo(out, fi, hashPath(child.fullPath()))
	ch := n.NewInode(ctx, child, child.stableAttr(fi))
	return ch, fs.OK
}

func (n *sftpNode) Unlink(ctx context.Context, name string) syscall.Errno {
	if !n.readWrite {
		return syscall.EROFS
	}
	return fs.ToErrno(n.client.Remove(pathpkg.Join(n.fullPath(), name)))
}

func (n *sftpNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	if !n.readWrite {
		return syscall.EROFS
	}
	return fs.ToErrno(n.client.RemoveDirectory(pathpkg.Join(n.fullPath(), name)))
}

func (n *sftpNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	if !n.readWrite {
		return syscall.EROFS
	}
	target, ok := newParent.(*sftpNode)
	if !ok {
		return syscall.EXDEV
	}

	oldPath := pathpkg.Join(n.fullPath(), name)
	newPath := pathpkg.Join(target.fullPath(), newName)
	if err := n.client.PosixRename(oldPath, newPath); err != nil {
		return fs.ToErrno(err)
	}
	return fs.OK
}

func (n *sftpNode) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if !n.readWrite {
		return nil, syscall.EROFS
	}

	child := n.childNode(name)
	if err := n.client.Symlink(target, child.fullPath()); err != nil {
		return nil, fs.ToErrno(err)
	}
	fi, err := n.client.Lstat(child.fullPath())
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	fillEntryFromFileInfo(out, fi, hashPath(child.fullPath()))
	ch := n.NewInode(ctx, child, child.stableAttr(fi))
	return ch, fs.OK
}

func (n *sftpNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	target, err := n.client.ReadLink(n.fullPath())
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	return []byte(target), fs.OK
}

func (n *sftpNode) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if !n.readWrite {
		return syscall.EROFS
	}

	path := n.fullPath()
	if mode, ok := in.GetMode(); ok {
		if err := n.client.Chmod(path, os.FileMode(mode)); err != nil {
			return fs.ToErrno(err)
		}
	}
	if uid, ok := in.GetUID(); ok {
		gid, _ := in.GetGID()
		if err := n.client.Chown(path, int(uid), int(gid)); err != nil {
			return fs.ToErrno(err)
		}
	}
	if size, ok := in.GetSize(); ok {
		if err := n.client.Truncate(path, int64(size)); err != nil {
			return fs.ToErrno(err)
		}
	}
	if atime, ok := in.GetATime(); ok {
		mtime, okM := in.GetMTime()
		if !okM {
			mtime = atime
		}
		if err := n.client.Chtimes(path, atime, mtime); err != nil {
			return fs.ToErrno(err)
		}
	}

	fi, err := n.client.Lstat(path)
	if err != nil {
		return fs.ToErrno(err)
	}
	fillAttrFromFileInfo(out, fi, hashPath(path))
	return fs.OK
}

type sftpFileHandle struct {
	file   *sftp.File
	client *sftp.Client
	path   string
}

func (f *sftpFileHandle) Release(ctx context.Context) syscall.Errno {
	if f.file == nil {
		return fs.OK
	}
	err := f.file.Close()
	f.file = nil
	return fs.ToErrno(err)
}

func (f *sftpFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	n, err := f.file.ReadAt(dest, off)
	if err != nil && err != io.EOF {
		return nil, fs.ToErrno(err)
	}
	return fuse.ReadResultData(dest[:n]), fs.OK
}

func (f *sftpFileHandle) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	n, err := f.file.WriteAt(data, off)
	if err != nil {
		return 0, fs.ToErrno(err)
	}
	return uint32(n), fs.OK
}

func (f *sftpFileHandle) Flush(ctx context.Context) syscall.Errno {
	return fs.ToErrno(f.file.Sync())
}

func (f *sftpFileHandle) Fsync(ctx context.Context, flags uint32) syscall.Errno {
	return fs.ToErrno(f.file.Sync())
}

func (f *sftpFileHandle) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	fi, err := f.file.Stat()
	if err != nil {
		return fs.ToErrno(err)
	}
	fillAttrFromFileInfo(out, fi, hashPath(f.path))
	return fs.OK
}

func (f *sftpFileHandle) Setattr(ctx context.Context, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if size, ok := in.GetSize(); ok {
		if err := f.file.Truncate(int64(size)); err != nil {
			return fs.ToErrno(err)
		}
	}
	if mode, ok := in.GetMode(); ok {
		if err := f.client.Chmod(f.path, os.FileMode(mode)); err != nil {
			return fs.ToErrno(err)
		}
	}
	return f.Getattr(ctx, out)
}

func fillAttrFromFileInfo(out *fuse.AttrOut, fi os.FileInfo, ino uint64) {
	out.Attr.Ino = ino
	out.Attr.Mode = uint32(fi.Mode())
	out.Attr.Size = uint64(fi.Size())
	if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
		out.Attr.FromStat(stat)
		out.Attr.Ino = ino
	}
}

func fillEntryFromFileInfo(out *fuse.EntryOut, fi os.FileInfo, ino uint64) {
	attrOut := &fuse.AttrOut{Attr: out.Attr}
	fillAttrFromFileInfo(attrOut, fi, ino)
	out.Attr = attrOut.Attr
	out.SetAttrTimeout(time.Second)
	out.SetEntryTimeout(time.Second)
}

func entryMode(mode os.FileMode) uint32 {
	switch {
	case mode.IsDir():
		return fuse.S_IFDIR
	case mode&os.ModeSymlink != 0:
		return fuse.S_IFLNK
	default:
		return fuse.S_IFREG
	}
}

func cleanRemotePath(path string) string {
	if path == "" {
		return "/"
	}
	cleaned := pathpkg.Clean(path)
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func hashPath(path string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(path))
	return h.Sum64()
}

func isWriteOpen(flags uint32) bool {
	accmode := flags & uint32(os.O_RDONLY|os.O_WRONLY|os.O_RDWR)
	return accmode == uint32(os.O_WRONLY) || accmode == uint32(os.O_RDWR)
}
