package smbfs

import (
	"context"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/absfs/smbfs/absfs"
)

// FileSystem implements absfs.FileSystem for SMB/CIFS network shares.
type FileSystem struct {
	config   *Config
	pool     *connectionPool
	pathNorm *pathNormalizer
	cache    *metadataCache
	ctx      context.Context
	cancel   context.CancelFunc
}

// Ensure FileSystem implements absfs.FileSystem.
var _ absfs.FileSystem = (*FileSystem)(nil)

// New creates a new SMB filesystem.
func New(config *Config) (*FileSystem, error) {
	if config == nil {
		return nil, ErrInvalidConfig
	}

	// Set defaults and validate
	config.setDefaults()
	if err := config.Validate(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	fs := &FileSystem{
		config:   config,
		pool:     newConnectionPool(config),
		pathNorm: newPathNormalizer(config.CaseSensitive),
		cache:    newMetadataCache(config.Cache),
		ctx:      ctx,
		cancel:   cancel,
	}

	// Start background cleanup
	fs.pool.startCleanup(ctx)

	return fs, nil
}

// Open opens a file for reading.
func (fsys *FileSystem) Open(name string) (absfs.File, error) {
	return fsys.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile opens a file with the specified flags and mode.
func (fsys *FileSystem) OpenFile(name string, flag int, perm fs.FileMode) (absfs.File, error) {
	// Validate and normalize path
	if err := validatePath(name); err != nil {
		return nil, wrapPathError("open", name, err)
	}

	name = fsys.pathNorm.normalize(name)
	smbPath := toSMBPath(name)

	var resultFile *File
	err := fsys.withRetry(fsys.ctx, func() error {
		// Get a connection from the pool
		conn, err := fsys.pool.get(fsys.ctx)
		if err != nil {
			return err
		}

		// Convert flags to os flags for go-smb2
		openFlag := flag
		if flag&os.O_CREATE != 0 {
			openFlag = flag
		}

		// Open the file
		file, err := conn.share.OpenFile(smbPath, openFlag, perm)
		if err != nil {
			fsys.pool.put(conn)
			return convertError(err)
		}

		resultFile = &File{
			fs:   fsys,
			conn: conn,
			file: file,
			path: name,
		}
		return nil
	})

	if err != nil {
		return nil, wrapPathError("open", name, err)
	}

	// Invalidate cache if file was created
	if flag&os.O_CREATE != 0 {
		fsys.cache.invalidate(name)
	}

	return resultFile, nil
}

// Create creates a new file for writing.
func (fsys *FileSystem) Create(name string) (absfs.File, error) {
	return fsys.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

// Stat returns file information.
func (fsys *FileSystem) Stat(name string) (fs.FileInfo, error) {
	if err := validatePath(name); err != nil {
		return nil, wrapPathError("stat", name, err)
	}

	name = fsys.pathNorm.normalize(name)

	// Check cache first
	if cachedInfo, ok := fsys.cache.getStatInfo(name); ok {
		return cachedInfo, nil
	}

	smbPath := toSMBPath(name)

	var info *fileInfo
	err := fsys.withRetry(fsys.ctx, func() error {
		conn, err := fsys.pool.get(fsys.ctx)
		if err != nil {
			return err
		}
		defer fsys.pool.put(conn)

		stat, err := conn.share.Stat(smbPath)
		if err != nil {
			return convertError(err)
		}

		info = &fileInfo{
			stat: stat,
			name: fsys.pathNorm.base(name),
		}
		return nil
	})

	if err != nil {
		return nil, wrapPathError("stat", name, err)
	}

	// Cache the result
	fsys.cache.putStatInfo(name, info)

	return info, nil
}

// Lstat returns file information (same as Stat for SMB).
func (fsys *FileSystem) Lstat(name string) (fs.FileInfo, error) {
	return fsys.Stat(name)
}

// ReadDir reads the directory and returns directory entries.
func (fsys *FileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := validatePath(name); err != nil {
		return nil, wrapPathError("readdir", name, err)
	}

	name = fsys.pathNorm.normalize(name)

	// Check cache first
	if cachedEntries, ok := fsys.cache.getDirEntries(name); ok {
		return cachedEntries, nil
	}

	f, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Check if it's a directory
	info, err := f.Stat()
	if err != nil {
		return nil, wrapPathError("readdir", name, err)
	}

	if !info.IsDir() {
		return nil, wrapPathError("readdir", name, ErrNotDirectory)
	}

	// Read directory entries
	file := f.(*File)
	entries, err := file.ReadDir(-1)
	if err != nil {
		return nil, err
	}

	// Cache the result
	fsys.cache.putDirEntries(name, entries)

	return entries, nil
}

// Mkdir creates a directory.
func (fsys *FileSystem) Mkdir(name string, perm fs.FileMode) error {
	if err := validatePath(name); err != nil {
		return wrapPathError("mkdir", name, err)
	}

	name = fsys.pathNorm.normalize(name)
	smbPath := toSMBPath(name)

	conn, err := fsys.pool.get(fsys.ctx)
	if err != nil {
		return wrapPathError("mkdir", name, err)
	}
	defer fsys.pool.put(conn)

	err = conn.share.Mkdir(smbPath, perm)
	if err != nil {
		return wrapPathError("mkdir", name, convertError(err))
	}

	// Invalidate cache for the new directory and its parent
	fsys.cache.invalidate(name)

	return nil
}

// MkdirAll creates a directory and all parent directories.
func (fsys *FileSystem) MkdirAll(name string, perm fs.FileMode) error {
	if err := validatePath(name); err != nil {
		return wrapPathError("mkdir", name, err)
	}

	name = fsys.pathNorm.normalize(name)

	// Check if directory already exists
	if stat, err := fsys.Stat(name); err == nil {
		if !stat.IsDir() {
			return wrapPathError("mkdir", name, ErrNotDirectory)
		}
		return nil
	}

	// Create parent directory first
	parent := fsys.pathNorm.dir(name)
	if parent != "/" && parent != "." {
		if err := fsys.MkdirAll(parent, perm); err != nil {
			return err
		}
	}

	// Create this directory
	return fsys.Mkdir(name, perm)
}

// Remove removes a file or empty directory.
func (fsys *FileSystem) Remove(name string) error {
	if err := validatePath(name); err != nil {
		return wrapPathError("remove", name, err)
	}

	name = fsys.pathNorm.normalize(name)
	smbPath := toSMBPath(name)

	conn, err := fsys.pool.get(fsys.ctx)
	if err != nil {
		return wrapPathError("remove", name, err)
	}
	defer fsys.pool.put(conn)

	err = conn.share.Remove(smbPath)
	if err != nil {
		return wrapPathError("remove", name, convertError(err))
	}

	// Invalidate cache for the removed file and its parent directory
	fsys.cache.invalidate(name)

	return nil
}

// RemoveAll removes a path and all children.
func (fsys *FileSystem) RemoveAll(name string) error {
	if err := validatePath(name); err != nil {
		return wrapPathError("remove", name, err)
	}

	name = fsys.pathNorm.normalize(name)

	// Check if it exists
	info, err := fsys.Stat(name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already removed
		}
		return err
	}

	// If it's a file, just remove it
	if !info.IsDir() {
		return fsys.Remove(name)
	}

	// Read directory contents
	entries, err := fsys.ReadDir(name)
	if err != nil {
		return err
	}

	// Remove all children first
	for _, entry := range entries {
		childPath := fsys.pathNorm.join(name, entry.Name())
		if err := fsys.RemoveAll(childPath); err != nil {
			return err
		}
	}

	// Remove the directory itself
	return fsys.Remove(name)
}

// Rename renames (moves) a file or directory.
func (fsys *FileSystem) Rename(oldname, newname string) error {
	if err := validatePath(oldname); err != nil {
		return wrapPathError("rename", oldname, err)
	}
	if err := validatePath(newname); err != nil {
		return wrapPathError("rename", newname, err)
	}

	oldname = fsys.pathNorm.normalize(oldname)
	newname = fsys.pathNorm.normalize(newname)

	oldSMBPath := toSMBPath(oldname)
	newSMBPath := toSMBPath(newname)

	conn, err := fsys.pool.get(fsys.ctx)
	if err != nil {
		return wrapPathError("rename", oldname, err)
	}
	defer fsys.pool.put(conn)

	err = conn.share.Rename(oldSMBPath, newSMBPath)
	if err != nil {
		return wrapPathError("rename", oldname, convertError(err))
	}

	// Invalidate cache for both old and new paths and their parent directories
	fsys.cache.invalidate(oldname)
	fsys.cache.invalidate(newname)

	return nil
}

// Chmod changes the mode of a file.
func (fsys *FileSystem) Chmod(name string, mode fs.FileMode) error {
	if err := validatePath(name); err != nil {
		return wrapPathError("chmod", name, err)
	}

	name = fsys.pathNorm.normalize(name)
	smbPath := toSMBPath(name)

	conn, err := fsys.pool.get(fsys.ctx)
	if err != nil {
		return wrapPathError("chmod", name, err)
	}
	defer fsys.pool.put(conn)

	err = conn.share.Chmod(smbPath, mode)
	if err != nil {
		return wrapPathError("chmod", name, convertError(err))
	}

	// Invalidate stat cache since metadata changed
	fsys.cache.invalidate(name)

	return nil
}

// Chown changes the owner of a file.
func (fsys *FileSystem) Chown(name string, uid, gid int) error {
	// SMB doesn't directly support Unix ownership
	// This would require SID manipulation which is complex
	return wrapPathError("chown", name, ErrNotImplemented)
}

// Chtimes changes the access and modification times of a file.
func (fsys *FileSystem) Chtimes(name string, atime, mtime time.Time) error {
	if err := validatePath(name); err != nil {
		return wrapPathError("chtimes", name, err)
	}

	name = fsys.pathNorm.normalize(name)
	smbPath := toSMBPath(name)

	conn, err := fsys.pool.get(fsys.ctx)
	if err != nil {
		return wrapPathError("chtimes", name, err)
	}
	defer fsys.pool.put(conn)

	err = conn.share.Chtimes(smbPath, atime, mtime)
	if err != nil {
		return wrapPathError("chtimes", name, convertError(err))
	}

	// Invalidate stat cache since metadata changed
	fsys.cache.invalidate(name)

	return nil
}

// TempDir returns the default directory for temporary files.
// For SMB filesystems, this returns "/tmp" which can be created on the share.
func (fsys *FileSystem) TempDir() string {
	return "/tmp"
}

// Truncate changes the size of the named file.
func (fsys *FileSystem) Truncate(name string, size int64) error {
	if err := validatePath(name); err != nil {
		return wrapPathError("truncate", name, err)
	}

	name = fsys.pathNorm.normalize(name)

	// Open the file for writing
	f, err := fsys.OpenFile(name, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	// Use the file's Truncate method
	file := f.(*File)
	err = file.Truncate(size)
	if err != nil {
		return wrapPathError("truncate", name, err)
	}

	// Invalidate cache since file size changed
	fsys.cache.invalidate(name)

	return nil
}

// Close closes the filesystem and releases all resources.
func (fsys *FileSystem) Close() error {
	fsys.cancel()
	return fsys.pool.Close()
}

// convertFlags converts os.O_* flags to SMB access mode and create disposition.
func convertFlags(flag int) (accessMode uint32, createDisposition uint32) {
	// Access mode
	switch flag & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR) {
	case os.O_RDONLY:
		accessMode = 0x80000000 // GENERIC_READ
	case os.O_WRONLY:
		accessMode = 0x40000000 // GENERIC_WRITE
	case os.O_RDWR:
		accessMode = 0xC0000000 // GENERIC_READ | GENERIC_WRITE
	}

	// Create disposition
	switch {
	case flag&os.O_CREATE != 0 && flag&os.O_EXCL != 0:
		createDisposition = 1 // CREATE_NEW (fail if exists)
	case flag&os.O_CREATE != 0 && flag&os.O_TRUNC != 0:
		createDisposition = 2 // CREATE_ALWAYS (overwrite)
	case flag&os.O_CREATE != 0:
		createDisposition = 4 // OPEN_ALWAYS (create if not exists)
	case flag&os.O_TRUNC != 0:
		createDisposition = 5 // TRUNCATE_EXISTING
	default:
		createDisposition = 3 // OPEN_EXISTING
	}

	return accessMode, createDisposition
}

// Chdir changes the current working directory.
// For SMB filesystems, this is not typically used as paths are absolute.
func (fsys *FileSystem) Chdir(dir string) error {
	// SMB doesn't have a concept of current working directory
	// We could track it internally if needed, but for now we just validate the path exists
	if err := validatePath(dir); err != nil {
		return wrapPathError("chdir", dir, err)
	}

	dir = fsys.pathNorm.normalize(dir)
	info, err := fsys.Stat(dir)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return wrapPathError("chdir", dir, ErrNotDirectory)
	}

	return nil
}

// Getwd returns the current working directory.
// For SMB filesystems, this always returns "/" as there's no real working directory concept.
func (fsys *FileSystem) Getwd() (string, error) {
	return "/", nil
}

// ReadFile reads the named file and returns its contents.
func (fsys *FileSystem) ReadFile(name string) ([]byte, error) {
	if err := validatePath(name); err != nil {
		return nil, wrapPathError("readfile", name, err)
	}

	name = fsys.pathNorm.normalize(name)

	f, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Get file size for efficient buffer allocation
	info, err := f.Stat()
	if err != nil {
		return nil, wrapPathError("readfile", name, err)
	}

	if info.IsDir() {
		return nil, wrapPathError("readfile", name, ErrNotDirectory)
	}

	// Allocate buffer based on file size
	size := info.Size()
	buf := make([]byte, size)

	// Read entire file
	n, err := io.ReadFull(f.(*File), buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, wrapPathError("readfile", name, err)
	}

	return buf[:n], nil
}

// Sub returns an fs.FS corresponding to the subtree rooted at dir.
func (fsys *FileSystem) Sub(dir string) (fs.FS, error) {
	if err := validatePath(dir); err != nil {
		return nil, wrapPathError("sub", dir, err)
	}

	dir = fsys.pathNorm.normalize(dir)

	// Use absfs.FilerToFS to create a read-only fs.FS
	return absfs.FilerToFS(fsys, dir)
}

// NewWithFactory creates a new SMB filesystem with a custom connection factory.
// This is primarily used for testing with mock connections.
func NewWithFactory(config *Config, factory ConnectionFactory) (*FileSystem, error) {
	if config == nil {
		return nil, ErrInvalidConfig
	}

	// Set defaults and validate
	config.setDefaults()
	if err := config.Validate(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	fs := &FileSystem{
		config:   config,
		pool:     newConnectionPoolWithFactory(config, factory),
		pathNorm: newPathNormalizer(config.CaseSensitive),
		cache:    newMetadataCache(config.Cache),
		ctx:      ctx,
		cancel:   cancel,
	}

	// Start background cleanup
	fs.pool.startCleanup(ctx)

	return fs, nil
}

// subFS represents a sub-filesystem rooted at a specific directory.
type subFS struct {
	parent *FileSystem
	root   string
}

// Ensure subFS implements absfs.Filer.
var _ absfs.Filer = (*subFS)(nil)

// joinPath joins the root path with the given path.
func (sub *subFS) joinPath(name string) string {
	if err := validatePath(name); err != nil {
		return name
	}
	// Normalize the name and join with root
	name = sub.parent.pathNorm.normalize(name)
	return sub.parent.pathNorm.join(sub.root, name)
}

// OpenFile opens a file within the sub-filesystem.
func (sub *subFS) OpenFile(name string, flag int, perm fs.FileMode) (absfs.File, error) {
	return sub.parent.OpenFile(sub.joinPath(name), flag, perm)
}

// Mkdir creates a directory within the sub-filesystem.
func (sub *subFS) Mkdir(name string, perm fs.FileMode) error {
	return sub.parent.Mkdir(sub.joinPath(name), perm)
}

// Remove removes a file or directory within the sub-filesystem.
func (sub *subFS) Remove(name string) error {
	return sub.parent.Remove(sub.joinPath(name))
}

// Rename renames a file within the sub-filesystem.
func (sub *subFS) Rename(oldpath, newpath string) error {
	return sub.parent.Rename(sub.joinPath(oldpath), sub.joinPath(newpath))
}

// Stat returns file information for a file within the sub-filesystem.
func (sub *subFS) Stat(name string) (fs.FileInfo, error) {
	return sub.parent.Stat(sub.joinPath(name))
}

// Chmod changes the mode of a file within the sub-filesystem.
func (sub *subFS) Chmod(name string, mode fs.FileMode) error {
	return sub.parent.Chmod(sub.joinPath(name), mode)
}

// Chtimes changes the access and modification times of a file within the sub-filesystem.
func (sub *subFS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return sub.parent.Chtimes(sub.joinPath(name), atime, mtime)
}

// Chown changes the owner of a file within the sub-filesystem.
func (sub *subFS) Chown(name string, uid, gid int) error {
	return sub.parent.Chown(sub.joinPath(name), uid, gid)
}

// ReadDir reads a directory within the sub-filesystem.
func (sub *subFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return sub.parent.ReadDir(sub.joinPath(name))
}

// ReadFile reads a file within the sub-filesystem.
func (sub *subFS) ReadFile(name string) ([]byte, error) {
	return sub.parent.ReadFile(sub.joinPath(name))
}

// Sub returns a sub-filesystem of this sub-filesystem.
func (sub *subFS) Sub(dir string) (fs.FS, error) {
	fullPath := sub.joinPath(dir)

	// Use absfs.FilerToFS to create a read-only fs.FS
	return absfs.FilerToFS(sub.parent, fullPath)
}
