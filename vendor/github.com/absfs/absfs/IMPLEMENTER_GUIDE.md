# absfs Implementer Guide

**Audience**: Developers implementing the `absfs.Filer` interface to create new filesystem implementations.

This guide provides comprehensive instructions for implementing a custom filesystem that integrates with the absfs ecosystem.

## Table of Contents

1. [Overview](#overview)
2. [Why Implement a Filer?](#why-implement-a-filer)
3. [The Filer Interface](#the-filer-interface)
4. [Implementation Steps](#implementation-steps)
5. [Method-by-Method Requirements](#method-by-method-requirements)
6. [Using ExtendFiler](#using-extendfiler)
7. [Path Handling Requirements](#path-handling-requirements)
8. [Error Handling Patterns](#error-handling-patterns)
9. [Security Best Practices](#security-best-practices)
10. [Testing Your Implementation](#testing-your-implementation)
11. [Performance Considerations](#performance-considerations)
12. [Examples](#examples)

---

## Overview

The absfs package uses interface segregation to make filesystem implementation as simple as possible. You only need to implement the **`Filer` interface** (11 methods) to create a functional filesystem. The `ExtendFiler` function then provides all additional `FileSystem` methods automatically.

### Key Principles

- **Minimal Implementation**: Only 11 methods required (8 core + 3 io/fs compatibility methods)
- **No Internal Dependencies**: Your implementation doesn't need to know about absfs internals
- **Composition-Friendly**: Your Filer works with all absfs composition patterns
- **Absolute Paths Only**: Filers only need to handle absolute paths

---

## Why Implement a Filer?

Common use cases for custom Filer implementations:

- **Remote Storage**: Access cloud storage (S3, Azure Blob, GCS) as a filesystem
- **Databases**: Expose key-value stores (Bolt, BadgerDB) as filesystems
- **Virtual Filesystems**: In-memory, cached, or layered filesystems
- **Network Protocols**: SFTP, FTP, WebDAV filesystem access
- **Testing**: Mock filesystems for unit testing
- **Composition**: Base implementations for wrapped/layered filesystems

---

## The Filer Interface

The minimum interface you must implement:

```go
type Filer interface {
    OpenFile(name string, flag int, perm os.FileMode) (File, error)
    Mkdir(name string, perm os.FileMode) error
    Remove(name string) error
    Rename(oldpath, newpath string) error
    Stat(name string) (os.FileInfo, error)
    Chmod(name string, mode os.FileMode) error
    Chtimes(name string, atime time.Time, mtime time.Time) error
    Chown(name string, uid, gid int) error

    // io/fs compatibility methods
    ReadDir(name string) ([]fs.DirEntry, error)
    ReadFile(name string) ([]byte, error)
    Sub(dir string) (fs.FS, error)
}
```

### File Interface

`OpenFile` must return an `absfs.File`, which is:

```go
type File interface {
    io.Closer
    io.Reader
    io.Writer
    Name() string
    Readdir(count int) ([]os.FileInfo, error)
    Readdirnames(n int) ([]string, error)
    Stat() (os.FileInfo, error)
    Sync() error
    Truncate(size int64) error
    WriteString(s string) (n int, err error)
}
```

For files that need seeking, also implement `io.Seeker`. Use `absfs.ExtendSeekable()` if your file only has `io.ReadSeeker` or `io.WriteSeeker`.

---

## Implementation Steps

### Step 1: Define Your Filer Struct

```go
package myfs

import (
    "os"
    "time"
    "github.com/absfs/absfs"
)

type MyFiler struct {
    // Your filesystem state here
    // Example: connection, cache, base path, etc.
}

func New() *MyFiler {
    return &MyFiler{
        // Initialize your state
    }
}
```

### Step 2: Implement All Filer Methods

Implement each of the 8 required methods. See [Method-by-Method Requirements](#method-by-method-requirements) for details on each method.

### Step 3: Implement the File Interface

```go
type myFile struct {
    name string
    // Your file state
}

func (f *myFile) Read(p []byte) (n int, err error) {
    // Implementation
}

func (f *myFile) Write(p []byte) (n int, err error) {
    // Implementation
}

// ... implement all File methods
```

### Step 4: Create a FileSystem Constructor

```go
func NewFS() absfs.FileSystem {
    return absfs.ExtendFiler(New())
}
```

---

## Method-by-Method Requirements

### OpenFile

```go
func (fs *MyFiler) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error)
```

**Purpose**: Open or create a file with specified flags and permissions.

**Requirements**:
- Handle `absfs.O_RDONLY`, `absfs.O_WRONLY`, `absfs.O_RDWR` access modes
- Support `absfs.O_CREATE`, `absfs.O_APPEND`, `absfs.O_TRUNC`, `absfs.O_EXCL` flags
- Create parent directories if `O_CREATE` is set (or return error if they don't exist)
- Return `os.ErrExist` if `O_CREATE|O_EXCL` and file exists
- Return `os.ErrNotExist` if file doesn't exist and `O_CREATE` not set
- Apply `perm` when creating new files
- Return a File that implements the `absfs.File` interface

**Path Handling**:
- `name` will always be an absolute path
- No need to handle relative paths

**Example**:
```go
func (fs *MyFiler) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error) {
    // Parse flags
    accessMode := flag & absfs.O_ACCESS
    create := (flag & absfs.O_CREATE) != 0
    excl := (flag & absfs.O_EXCL) != 0
    trunc := (flag & absfs.O_TRUNC) != 0
    append := (flag & absfs.O_APPEND) != 0

    // Check if file exists
    exists := fs.fileExists(name)

    // Handle O_CREATE|O_EXCL
    if create && excl && exists {
        return nil, os.ErrExist
    }

    // Handle file doesn't exist
    if !exists {
        if !create {
            return nil, os.ErrNotExist
        }
        return fs.createFile(name, perm)
    }

    // Open existing file
    return fs.openExisting(name, accessMode, trunc, append)
}
```

---

### Mkdir

```go
func (fs *MyFiler) Mkdir(name string, perm os.FileMode) error
```

**Purpose**: Create a single directory.

**Requirements**:
- Create only the final directory component
- Return `os.ErrNotExist` if parent doesn't exist
- Return `os.ErrExist` if directory already exists
- Apply `perm` to the new directory
- Fails if `name` already exists as a file

**Path Handling**:
- `name` will always be an absolute path
- Use `filepath.Dir(name)` to check if parent exists

**Example**:
```go
func (fs *MyFiler) Mkdir(name string, perm os.FileMode) error {
    // Check if already exists
    if fs.exists(name) {
        return os.ErrExist
    }

    // Check if parent exists
    parent := filepath.Dir(name)
    if parent != name && !fs.dirExists(parent) {
        return os.ErrNotExist
    }

    // Create directory
    return fs.createDirectory(name, perm)
}
```

---

### Remove

```go
func (fs *MyFiler) Remove(name string) error
```

**Purpose**: Remove a file or empty directory.

**Requirements**:
- Remove files completely
- Remove empty directories
- Return error if directory is not empty
- Return `os.ErrNotExist` if path doesn't exist
- After successful removal, Stat(name) should return `os.ErrNotExist`

**Important**: Only removes a single file or empty directory. Does NOT recursively remove directory trees (that's `RemoveAll`'s job, which is provided by `ExtendFiler`).

**Example**:
```go
func (fs *MyFiler) Remove(name string) error {
    info, err := fs.Stat(name)
    if err != nil {
        return err // Typically os.ErrNotExist
    }

    if info.IsDir() {
        // Check if directory is empty
        if !fs.isEmpty(name) {
            return errors.New("directory not empty")
        }
    }

    return fs.delete(name)
}
```

---

### Rename

```go
func (fs *MyFiler) Rename(oldpath, newpath string) error
```

**Purpose**: Rename or move a file or directory.

**Requirements**:
- Move/rename `oldpath` to `newpath`
- Can move across directories within the filesystem
- If `newpath` exists, behavior is platform-specific (usually replaces)
- Return `os.ErrNotExist` if `oldpath` doesn't exist
- Atomic operation where possible
- Works for both files and directories

**Example**:
```go
func (fs *MyFiler) Rename(oldpath, newpath string) error {
    // Check if oldpath exists
    if !fs.exists(oldpath) {
        return os.ErrNotExist
    }

    // Check if newpath parent exists
    parent := filepath.Dir(newpath)
    if !fs.dirExists(parent) {
        return os.ErrNotExist
    }

    // Perform rename
    return fs.move(oldpath, newpath)
}
```

---

### Stat

```go
func (fs *MyFiler) Stat(name string) (os.FileInfo, error)
```

**Purpose**: Return file information.

**Requirements**:
- Return `os.FileInfo` with accurate information
- Return `os.ErrNotExist` if path doesn't exist
- For symbolic links, return info about the target (follow links)

**FileInfo must provide**:
- `Name()`: base name of the file
- `Size()`: length in bytes
- `Mode()`: file mode bits (including type bits for directories)
- `ModTime()`: modification time
- `IsDir()`: true if directory
- `Sys()`: underlying data source (can be nil)

**Example**:
```go
func (fs *MyFiler) Stat(name string) (os.FileInfo, error) {
    entry, exists := fs.lookup(name)
    if !exists {
        return nil, os.ErrNotExist
    }

    return &fileInfo{
        name:    filepath.Base(name),
        size:    entry.Size,
        mode:    entry.Mode,
        modTime: entry.ModTime,
        isDir:   entry.IsDir,
    }, nil
}
```

---

### Chmod

```go
func (fs *MyFiler) Chmod(name string, mode os.FileMode) error
```

**Purpose**: Change file mode/permissions.

**Requirements**:
- Change permission bits of file or directory
- Return `os.ErrNotExist` if path doesn't exist
- Only permission bits (0777) typically changeable
- Mode type bits (directory, symlink, etc.) are typically not changed

**Note**: Some filesystems don't support permissions (e.g., memory, some cloud storage). These can:
- Store the mode but ignore it
- Return nil (no-op)
- Return an error

**Example**:
```go
func (fs *MyFiler) Chmod(name string, mode os.FileMode) error {
    entry, exists := fs.lookup(name)
    if !exists {
        return os.ErrNotExist
    }

    // Update only permission bits
    entry.Mode = (entry.Mode &^ 0777) | (mode & 0777)
    return nil
}
```

---

### Chtimes

```go
func (fs *MyFiler) Chtimes(name string, atime time.Time, mtime time.Time) error
```

**Purpose**: Change file access and modification times.

**Requirements**:
- Update access time (`atime`) and modification time (`mtime`)
- Return `os.ErrNotExist` if path doesn't exist
- Both times should be updated atomically if possible

**Note**: Some filesystems only support modification time. These can store only `mtime` and ignore `atime`.

**Example**:
```go
func (fs *MyFiler) Chtimes(name string, atime time.Time, mtime time.Time) error {
    entry, exists := fs.lookup(name)
    if !exists {
        return os.ErrNotExist
    }

    entry.AccessTime = atime
    entry.ModTime = mtime
    return nil
}
```

---

### Chown

```go
func (fs *MyFiler) Chown(name string, uid, gid int) error
```

**Purpose**: Change file owner and group.

**Requirements**:
- Change user ID (`uid`) and group ID (`gid`) ownership
- Return `os.ErrNotExist` if path doesn't exist
- Pass -1 for `uid` or `gid` to leave that value unchanged

**Note**: Most virtual filesystems don't support ownership. These should:
- Return nil (no-op) if ownership doesn't apply
- Store the values if they might be used by consumers
- Return an error if ownership is required but not supported

**Example**:
```go
func (fs *MyFiler) Chown(name string, uid, gid int) error {
    // Many implementations can just return nil (no-op)
    // if ownership doesn't apply to the filesystem
    return nil
}
```

---

### ReadDir

```go
func (fs *MyFiler) ReadDir(name string) ([]fs.DirEntry, error)
```

**Purpose**: Read directory contents and return entries compatible with `io/fs`.

**Requirements**:
- Return list of directory entries sorted by filename
- Return `os.ErrNotExist` if directory doesn't exist
- Return error if `name` is not a directory
- Each `fs.DirEntry` must provide Name(), IsDir(), Type(), and Info() methods
- Compatible with `io/fs.ReadDirFS` interface

**Note**: `ExtendFiler` provides a default implementation that uses `Open()` and `File.ReadDir()`. You can implement this method directly for better performance.

**Example**:
```go
func (fs *MyFiler) ReadDir(name string) ([]fs.DirEntry, error) {
    entries, exists := fs.lookupDir(name)
    if !exists {
        return nil, os.ErrNotExist
    }

    // Convert internal entries to fs.DirEntry
    result := make([]fs.DirEntry, 0, len(entries))
    for _, entry := range entries {
        result = append(result, &dirEntry{
            name:  entry.Name,
            isDir: entry.IsDir,
            mode:  entry.Mode,
            // ... other fields
        })
    }

    // Sort by name
    sort.Slice(result, func(i, j int) bool {
        return result[i].Name() < result[j].Name()
    })

    return result, nil
}

// Example fs.DirEntry implementation
type dirEntry struct {
    name  string
    isDir bool
    mode  os.FileMode
    // ... other fields
}

func (d *dirEntry) Name() string               { return d.name }
func (d *dirEntry) IsDir() bool                { return d.isDir }
func (d *dirEntry) Type() fs.FileMode          { return d.mode.Type() }
func (d *dirEntry) Info() (fs.FileInfo, error) { /* return full FileInfo */ }
```

---

### ReadFile

```go
func (fs *MyFiler) ReadFile(name string) ([]byte, error)
```

**Purpose**: Read entire file contents in one operation.

**Requirements**:
- Read and return entire file contents
- Return `os.ErrNotExist` if file doesn't exist
- Return error if `name` is a directory
- Compatible with `io/fs.ReadFileFS` interface

**Note**: `ExtendFiler` provides a default implementation that uses `Open()` and reads the file. You can implement this method directly for better performance (e.g., direct buffer access).

**Example**:
```go
func (fs *MyFiler) ReadFile(name string) ([]byte, error) {
    entry, exists := fs.lookup(name)
    if !exists {
        return nil, os.ErrNotExist
    }

    if entry.IsDir {
        return nil, errors.New("is a directory")
    }

    // For in-memory filesystems, might return data directly
    return append([]byte(nil), entry.Data...), nil

    // For other filesystems, open and read
    // f, err := fs.Open(name)
    // if err != nil {
    //     return nil, err
    // }
    // defer f.Close()
    // return io.ReadAll(f)
}
```

---

### Sub

```go
func (fs *MyFiler) Sub(dir string) (fs.FS, error)
```

**Purpose**: Return a read-only `fs.FS` view of a subdirectory.

**Requirements**:
- Return an `fs.FS` rooted at `dir`
- Return `os.ErrNotExist` if directory doesn't exist
- Return error if `dir` is not a directory
- The returned `fs.FS` must be read-only
- Paths in the returned `fs.FS` are relative to `dir`
- Compatible with `io/fs.SubFS` interface

**Note**: `ExtendFiler` provides a default implementation using `FilerToFS()`. Most implementations don't need to override this unless they have specialized subtree handling.

**Example**:
```go
func (fs *MyFiler) Sub(dir string) (fs.FS, error) {
    // Verify directory exists
    info, err := fs.Stat(dir)
    if err != nil {
        return nil, err
    }
    if !info.IsDir() {
        return nil, errors.New("not a directory")
    }

    // Use the default implementation
    return absfs.FilerToFS(fs, dir)
}
```

**Custom Implementation** (advanced):
```go
// Only if you need specialized behavior
type subFS struct {
    parent *MyFiler
    root   string
}

func (fs *MyFiler) Sub(dir string) (fs.FS, error) {
    info, err := fs.Stat(dir)
    if err != nil {
        return nil, err
    }
    if !info.IsDir() {
        return nil, errors.New("not a directory")
    }

    return &subFS{parent: fs, root: dir}, nil
}

func (s *subFS) Open(name string) (fs.File, error) {
    // Validate per fs.ValidPath
    if !fs.ValidPath(name) {
        return nil, fs.ErrInvalid
    }
    fullPath := path.Join(s.root, name)
    return s.parent.OpenFile(fullPath, os.O_RDONLY, 0)
}
```

---

## Using ExtendFiler

Once you've implemented the `Filer` interface, use `ExtendFiler` to get a full `FileSystem`:

```go
func NewFS() absfs.FileSystem {
    filer := New()
    return absfs.ExtendFiler(filer)
}
```

### What ExtendFiler Provides

`ExtendFiler` adds these `FileSystem` methods automatically:

- **`TempDir()`**: Returns `/tmp` for virtual filesystems
- **`Chdir(dir)`**: Changes current working directory for this FileSystem instance
- **`Getwd()`**: Returns current working directory
- **`Open(name)`**: Convenience for `OpenFile(name, O_RDONLY, 0)`
- **`Create(name)`**: Convenience for `OpenFile(name, O_RDWR|O_CREATE|O_TRUNC, 0666)`
- **`MkdirAll(path, perm)`**: Recursively creates directories
- **`RemoveAll(path)`**: Recursively removes directories and contents
- **`Truncate(name, size)`**: Truncates file to specified size

### Path Resolution

`ExtendFiler` handles path resolution for you:
- Maintains a current working directory (`cwd`) per FileSystem instance
- Converts relative paths to absolute paths
- Your Filer methods always receive absolute paths

### Optional: Override Default Implementations

If you need custom implementations of any `FileSystem` methods for performance or functionality, just implement them on your Filer. `ExtendFiler` will detect and use your implementation instead of the default.

Example:
```go
// Custom MkdirAll for better performance
func (fs *MyFiler) MkdirAll(path string, perm os.FileMode) error {
    // Your optimized implementation
}
```

---

## Path Handling Requirements

### Absolute Paths Only

Your Filer implementation only needs to handle **absolute paths**. `ExtendFiler` converts relative paths to absolute before calling your methods.

**What counts as absolute:**
- Unix: Paths starting with `/`  (e.g., `/home/user/file.txt`)
- Windows: Paths with drive letters (e.g., `C:\Users\file.txt`) or UNC paths (e.g., `\\server\share`)

### Virtual-Absolute Paths

For maximum portability, absfs treats paths starting with `/` or `\` as "virtual-absolute" - they work consistently across platforms. Your Filer can:

**Option 1**: Accept virtual-absolute paths directly
```go
// Works on Unix AND Windows
fs.Create("/config/app.json")
```

**Option 2**: Map to OS-specific paths (for OS filesystem implementations)
```go
// Translate /config -> C:\config on Windows
```

### Path Cleaning

You can assume paths are already cleaned (no `..`, `.`, or double separators) if users follow best practices, but it's safer to clean paths yourself:

```go
name = filepath.Clean(name)
```

### Cross-Platform Considerations

If your filesystem should work across platforms:
- Use `filepath.Join()` to construct paths
- Use `filepath.Dir()` and `filepath.Base()` to parse paths
- Use `filepath.Separator` for the path separator
- Don't assume specific separators (`/` or `\`)

---

## Error Handling Patterns

### Standard Errors

Use standard `os` package errors when appropriate:

```go
import "os"

// File not found
return os.ErrNotExist

// File already exists
return os.ErrExist

// Permission denied
return os.ErrPermission

// Invalid argument
return os.ErrInvalid

// Operation not supported
return os.ErrNotSupported
```

### Custom Errors

For filesystem-specific errors:

```go
import "fmt"

return fmt.Errorf("myfs: connection failed: %w", err)
```

### Error Consistency

Be consistent with error returns:
- `Stat`, `Remove`, `Rename`, `Chmod`, `Chtimes`, `Chown`: Return `os.ErrNotExist` if path doesn't exist
- `OpenFile`: Return `os.ErrNotExist` if file doesn't exist and `O_CREATE` not set
- `Mkdir`: Return `os.ErrExist` if directory exists, `os.ErrNotExist` if parent doesn't exist

---

## Security Best Practices

### Input Validation

**Validate all path inputs:**

```go
func (fs *MyFiler) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error) {
    // Reject empty paths
    if name == "" {
        return nil, os.ErrInvalid
    }

    // Reject paths with null bytes
    if strings.Contains(name, "\x00") {
        return nil, os.ErrInvalid
    }

    // Clean path to prevent traversal
    name = filepath.Clean(name)

    // Your implementation...
}
```

### Permission Enforcement

**Respect permission bits:**

```go
func (fs *MyFiler) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error) {
    info, err := fs.Stat(name)
    if err == nil {
        // File exists - check if we have access based on mode
        if (flag&absfs.O_WRONLY != 0 || flag&absfs.O_RDWR != 0) && info.Mode()&0200 == 0 {
            return nil, os.ErrPermission
        }
    }
    // ... rest of implementation
}
```

### Resource Limits

**Prevent resource exhaustion:**

```go
type MyFiler struct {
    openFiles   int
    maxOpenFiles int
    sync.Mutex
}

func (fs *MyFiler) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error) {
    fs.Lock()
    if fs.openFiles >= fs.maxOpenFiles {
        fs.Unlock()
        return nil, errors.New("too many open files")
    }
    fs.openFiles++
    fs.Unlock()

    // ... open file ...
}
```

### Sanitize Paths for Security

If your filesystem interacts with OS files or external systems, sanitize paths:

```go
// Prevent path traversal
name = filepath.Clean(name)

// Ensure path stays within bounds
if strings.Contains(name, "..") {
    return nil, os.ErrPermission
}
```

### Secrets and Credentials

- Never log passwords, tokens, or keys
- Use secure storage for credentials
- Support environment variables for configuration
- Clear sensitive data when done

---

## Testing Your Implementation

### Basic Functionality Tests

Test all Filer methods:

```go
func TestMyFiler(t *testing.T) {
    fs := New()

    // Test file creation
    f, err := fs.OpenFile("/test.txt", absfs.O_CREATE|absfs.O_RDWR, 0644)
    if err != nil {
        t.Fatalf("OpenFile failed: %v", err)
    }
    defer f.Close()

    // Test write
    n, err := f.Write([]byte("hello"))
    if err != nil {
        t.Fatalf("Write failed: %v", err)
    }
    if n != 5 {
        t.Errorf("wrote %d bytes, want 5", n)
    }

    // Test stat
    info, err := fs.Stat("/test.txt")
    if err != nil {
        t.Fatalf("Stat failed: %v", err)
    }
    if info.Size() != 5 {
        t.Errorf("size = %d, want 5", info.Size())
    }

    // Test remove
    if err := fs.Remove("/test.txt"); err != nil {
        t.Fatalf("Remove failed: %v", err)
    }
}
```

### Error Condition Tests

Test error cases:

```go
func TestErrors(t *testing.T) {
    fs := New()

    // Test file not found
    _, err := fs.OpenFile("/nonexistent.txt", absfs.O_RDONLY, 0)
    if !errors.Is(err, os.ErrNotExist) {
        t.Errorf("expected ErrNotExist, got %v", err)
    }

    // Test create exclusive
    fs.OpenFile("/exists.txt", absfs.O_CREATE|absfs.O_RDWR, 0644)
    _, err = fs.OpenFile("/exists.txt", absfs.O_CREATE|absfs.O_EXCL|absfs.O_RDWR, 0644)
    if !errors.Is(err, os.ErrExist) {
        t.Errorf("expected ErrExist, got %v", err)
    }
}
```

### FileSystem Interface Tests

Test with `ExtendFiler`:

```go
func TestFileSystem(t *testing.T) {
    fs := absfs.ExtendFiler(New())

    // Test MkdirAll
    if err := fs.MkdirAll("/a/b/c", 0755); err != nil {
        t.Fatalf("MkdirAll failed: %v", err)
    }

    // Test Chdir/Getwd
    if err := fs.Chdir("/a/b"); err != nil {
        t.Fatalf("Chdir failed: %v", err)
    }
    cwd, err := fs.Getwd()
    if err != nil {
        t.Fatalf("Getwd failed: %v", err)
    }
    if cwd != "/a/b" {
        t.Errorf("cwd = %q, want %q", cwd, "/a/b")
    }
}
```

### Concurrent Access Tests

Test if your implementation is thread-safe at the Filer level:

```go
func TestConcurrency(t *testing.T) {
    fs := New()

    // Create separate FileSystem instances for each goroutine
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            // Each goroutine should get its own FileSystem
            localFS := absfs.ExtendFiler(fs)
            file := fmt.Sprintf("/test%d.txt", id)
            f, _ := localFS.Create(file)
            f.Write([]byte("test"))
            f.Close()
        }(i)
    }
    wg.Wait()
}
```

---

## Performance Considerations

### Minimize System Calls

Batch operations when possible:

```go
// Bad: Multiple stats
for _, name := range files {
    info, _ := fs.Stat(name)
    // process info
}

// Better: Readdir once
infos, _ := dir.Readdir(-1)
for _, info := range infos {
    // process info
}
```

### Cache Frequently Accessed Data

```go
type MyFiler struct {
    statCache map[string]cachedStat
    cacheMu   sync.RWMutex
}

func (fs *MyFiler) Stat(name string) (os.FileInfo, error) {
    // Check cache first
    fs.cacheMu.RLock()
    cached, ok := fs.statCache[name]
    fs.cacheMu.RUnlock()

    if ok && time.Since(cached.time) < cacheTimeout {
        return cached.info, nil
    }

    // Fetch and cache
    info, err := fs.fetchStat(name)
    if err == nil {
        fs.cacheMu.Lock()
        fs.statCache[name] = cachedStat{info: info, time: time.Now()}
        fs.cacheMu.Unlock()
    }
    return info, err
}
```

### Pool Buffers

```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 32*1024)
    },
}

func (f *myFile) Read(p []byte) (n int, err error) {
    buf := bufferPool.Get().([]byte)
    defer bufferPool.Put(buf)
    // Use buf for temporary operations
}
```

---

## Examples

### Minimal Memory Filesystem

```go
package memfs

import (
    "bytes"
    "errors"
    "os"
    "path/filepath"
    "sync"
    "time"
    "github.com/absfs/absfs"
)

type MemFiler struct {
    files map[string]*memFile
    mu    sync.RWMutex
}

type memFile struct {
    data    []byte
    mode    os.FileMode
    modTime time.Time
    isDir   bool
}

func New() *MemFiler {
    return &MemFiler{
        files: make(map[string]*memFile),
    }
}

func (fs *MemFiler) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error) {
    fs.mu.Lock()
    defer fs.mu.Unlock()

    mf, exists := fs.files[name]
    create := (flag & absfs.O_CREATE) != 0

    if !exists && !create {
        return nil, os.ErrNotExist
    }

    if !exists {
        mf = &memFile{
            data:    []byte{},
            mode:    perm,
            modTime: time.Now(),
        }
        fs.files[name] = mf
    }

    return &openMemFile{
        name: name,
        file: mf,
        buf:  bytes.NewBuffer(mf.data),
    }, nil
}

func (fs *MemFiler) Mkdir(name string, perm os.FileMode) error {
    fs.mu.Lock()
    defer fs.mu.Unlock()

    if _, exists := fs.files[name]; exists {
        return os.ErrExist
    }

    fs.files[name] = &memFile{
        mode:    perm | os.ModeDir,
        modTime: time.Now(),
        isDir:   true,
    }
    return nil
}

func (fs *MemFiler) Remove(name string) error {
    fs.mu.Lock()
    defer fs.mu.Unlock()

    mf, exists := fs.files[name]
    if !exists {
        return os.ErrNotExist
    }

    if mf.isDir {
        // Check if empty
        prefix := name + "/"
        for k := range fs.files {
            if k != name && filepath.Dir(k) == name {
                return errors.New("directory not empty")
            }
        }
    }

    delete(fs.files, name)
    return nil
}

func (fs *MemFiler) Rename(oldpath, newpath string) error {
    fs.mu.Lock()
    defer fs.mu.Unlock()

    mf, exists := fs.files[oldpath]
    if !exists {
        return os.ErrNotExist
    }

    fs.files[newpath] = mf
    delete(fs.files, oldpath)
    return nil
}

func (fs *MemFiler) Stat(name string) (os.FileInfo, error) {
    fs.mu.RLock()
    defer fs.mu.RUnlock()

    mf, exists := fs.files[name]
    if !exists {
        return nil, os.ErrNotExist
    }

    return &fileInfo{
        name:    filepath.Base(name),
        size:    int64(len(mf.data)),
        mode:    mf.mode,
        modTime: mf.modTime,
        isDir:   mf.isDir,
    }, nil
}

func (fs *MemFiler) Chmod(name string, mode os.FileMode) error {
    fs.mu.Lock()
    defer fs.mu.Unlock()

    mf, exists := fs.files[name]
    if !exists {
        return os.ErrNotExist
    }

    mf.mode = mode
    return nil
}

func (fs *MemFiler) Chtimes(name string, atime time.Time, mtime time.Time) error {
    fs.mu.Lock()
    defer fs.mu.Unlock()

    mf, exists := fs.files[name]
    if !exists {
        return os.ErrNotExist
    }

    mf.modTime = mtime
    return nil
}

func (fs *MemFiler) Chown(name string, uid, gid int) error {
    // No-op for memory filesystem
    return nil
}

// Create FileSystem
func NewFS() absfs.FileSystem {
    return absfs.ExtendFiler(New())
}
```

---

## Additional Resources

- [absfs README](README.md) - Package overview and features
- [Architecture Guide](ARCHITECTURE.md) - Design decisions and patterns
- [Path Handling Guide](PATH_HANDLING.md) - Cross-platform path semantics
- [Security Policy](SECURITY.md) - Security considerations
- [GoDoc](https://pkg.go.dev/github.com/absfs/absfs) - API reference

---

## Getting Help

- Open an issue: https://github.com/absfs/absfs/issues
- Review existing implementations:
  - [osfs](https://github.com/absfs/osfs) - OS filesystem wrapper
  - [memfs](https://github.com/absfs/memfs) - In-memory filesystem
  - [nilfs](https://github.com/absfs/nilfs) - No-op filesystem

---

## Contributing Your Implementation

Once you've created a Filer implementation:

1. Publish it as a separate package/repo
2. Add comprehensive tests
3. Document usage in your README
4. Open a PR to add it to the absfs README ecosystem list

We'd love to link to your implementation!
