# absfs User Guide

**Audience**: Developers using absfs to work with filesystems in their Go applications.

This guide provides comprehensive instructions for using the absfs package to work with filesystems in a portable, testable way.

## Table of Contents

1. [Getting Started](#getting-started)
2. [Core Concepts](#core-concepts)
3. [Working with Files](#working-with-files)
4. [Working with Directories](#working-with-directories)
5. [io/fs Compatibility Methods](#iofs-compatibility-methods)
6. [Path Handling](#path-handling)
7. [Thread Safety](#thread-safety)
8. [Available Filesystems](#available-filesystems)
9. [Composition Patterns](#composition-patterns)
10. [Testing with absfs](#testing-with-absfs)
11. [Best Practices](#best-practices)

---

## Getting Started

### Installation

```bash
go get github.com/absfs/absfs
```

### Basic Example

```go
package main

import (
    "fmt"
    "github.com/absfs/absfs"
    "github.com/absfs/osfs"
)

func main() {
    // Create a filesystem
    fs := osfs.NewFS()

    // Create a file
    file, err := fs.Create("/tmp/hello.txt")
    if err != nil {
        panic(err)
    }
    defer file.Close()

    // Write to the file
    _, err = file.Write([]byte("Hello, absfs!"))
    if err != nil {
        panic(err)
    }

    fmt.Println("File created successfully!")
}
```

---

## Core Concepts

### FileSystem Interface

The `FileSystem` interface is your main entry point for filesystem operations:

```go
type FileSystem interface {
    // File operations
    Open(name string) (File, error)
    Create(name string) (File, error)
    OpenFile(name string, flag int, perm os.FileMode) (File, error)

    // Directory operations
    Mkdir(name string, perm os.FileMode) error
    MkdirAll(name string, perm os.FileMode) error
    Remove(name string) error
    RemoveAll(path string) error
    Rename(oldpath, newpath string) error

    // File information
    Stat(name string) (os.FileInfo, error)

    // File modification
    Chmod(name string, mode os.FileMode) error
    Chtimes(name string, atime time.Time, mtime time.Time) error
    Chown(name string, uid, gid int) error
    Truncate(name string, size int64) error

    // Path operations
    Chdir(dir string) error
    Getwd() (dir string, err error)
    TempDir() string
}
```

### File Interface

Files returned by filesystem operations implement:

```go
type File interface {
    io.Closer
    io.Reader
    io.Writer
    io.Seeker  // If the file supports seeking

    Name() string
    Readdir(count int) ([]os.FileInfo, error)
    Readdirnames(n int) ([]string, error)
    Stat() (os.FileInfo, error)
    Sync() error
    Truncate(size int64) error
    WriteString(s string) (n int, err error)
}
```

---

## Working with Files

### Creating Files

```go
// Simple file creation
file, err := fs.Create("/path/to/file.txt")
if err != nil {
    return err
}
defer file.Close()
```

### Opening Files

```go
// Open for reading
file, err := fs.Open("/path/to/file.txt")
if err != nil {
    return err
}
defer file.Close()

// Read the contents
data, err := io.ReadAll(file)
if err != nil {
    return err
}
fmt.Println(string(data))
```

### Opening with Specific Flags

```go
// Open for reading and writing
file, err := fs.OpenFile("/path/to/file.txt",
    absfs.O_RDWR|absfs.O_CREATE|absfs.O_TRUNC,
    0644)
if err != nil {
    return err
}
defer file.Close()
```

#### Available Flags

- `absfs.O_RDONLY`: Open for reading only
- `absfs.O_WRONLY`: Open for writing only
- `absfs.O_RDWR`: Open for reading and writing
- `absfs.O_CREATE`: Create file if it doesn't exist
- `absfs.O_EXCL`: Exclusive create (fails if file exists, use with O_CREATE)
- `absfs.O_TRUNC`: Truncate file to zero length when opening
- `absfs.O_APPEND`: Append mode (writes go to end of file)
- `absfs.O_SYNC`: Synchronous I/O

### Reading Files

```go
file, err := fs.Open("/path/to/file.txt")
if err != nil {
    return err
}
defer file.Close()

// Read in chunks
buffer := make([]byte, 1024)
for {
    n, err := file.Read(buffer)
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    // Process buffer[:n]
}
```

### Writing Files

```go
file, err := fs.Create("/path/to/file.txt")
if err != nil {
    return err
}
defer file.Close()

// Write bytes
_, err = file.Write([]byte("Hello, World!"))
if err != nil {
    return err
}

// Or write string
_, err = file.WriteString("More text\n")
if err != nil {
    return err
}

// Sync to storage
if err := file.Sync(); err != nil {
    return err
}
```

### Copying Files

```go
func copyFile(fs absfs.FileSystem, src, dst string) error {
    // Open source
    srcFile, err := fs.Open(src)
    if err != nil {
        return err
    }
    defer srcFile.Close()

    // Create destination
    dstFile, err := fs.Create(dst)
    if err != nil {
        return err
    }
    defer dstFile.Close()

    // Copy contents
    _, err = io.Copy(dstFile, srcFile)
    if err != nil {
        return err
    }

    // Sync to storage
    return dstFile.Sync()
}
```

### Getting File Information

```go
info, err := fs.Stat("/path/to/file.txt")
if err != nil {
    return err
}

fmt.Printf("Name: %s\n", info.Name())
fmt.Printf("Size: %d bytes\n", info.Size())
fmt.Printf("Mode: %s\n", info.Mode())
fmt.Printf("Modified: %s\n", info.ModTime())
fmt.Printf("IsDir: %v\n", info.IsDir())
```

### Modifying File Attributes

```go
// Change permissions
err := fs.Chmod("/path/to/file.txt", 0644)

// Change modification time
now := time.Now()
err = fs.Chtimes("/path/to/file.txt", now, now)

// Change ownership (Unix-like systems)
err = fs.Chown("/path/to/file.txt", uid, gid)

// Truncate file
err = fs.Truncate("/path/to/file.txt", 100) // Truncate to 100 bytes
```

### Removing Files

```go
// Remove a single file
err := fs.Remove("/path/to/file.txt")
if err != nil {
    return err
}
```

### Renaming/Moving Files

```go
// Rename or move a file
err := fs.Rename("/old/path/file.txt", "/new/path/file.txt")
if err != nil {
    return err
}
```

---

## Working with Directories

### Creating Directories

```go
// Create a single directory
err := fs.Mkdir("/path/to/dir", 0755)
if err != nil {
    return err
}

// Create directory hierarchy
err = fs.MkdirAll("/path/to/nested/dirs", 0755)
if err != nil {
    return err
}
```

### Listing Directory Contents

```go
// Open directory as a file
dir, err := fs.Open("/path/to/dir")
if err != nil {
    return err
}
defer dir.Close()

// Read all entries
infos, err := dir.Readdir(-1) // -1 means read all
if err != nil {
    return err
}

for _, info := range infos {
    fmt.Printf("%s (%d bytes)\n", info.Name(), info.Size())
}
```

### Walking Directory Trees

```go
import "path/filepath"

func walkDir(fs absfs.FileSystem, root string) error {
    return walk(fs, root, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        fmt.Printf("Visiting: %s\n", path)
        return nil
    })
}

func walk(fs absfs.FileSystem, path string, fn filepath.WalkFunc) error {
    info, err := fs.Stat(path)
    if err != nil {
        return fn(path, nil, err)
    }

    if err := fn(path, info, nil); err != nil {
        return err
    }

    if !info.IsDir() {
        return nil
    }

    dir, err := fs.Open(path)
    if err != nil {
        return err
    }
    defer dir.Close()

    names, err := dir.Readdirnames(-1)
    if err != nil {
        return err
    }

    for _, name := range names {
        fullPath := filepath.Join(path, name)
        if err := walk(fs, fullPath, fn); err != nil {
            return err
}
    }

    return nil
}
```

### Removing Directories

```go
// Remove empty directory
err := fs.Remove("/path/to/empty/dir")

// Remove directory and all contents recursively
err = fs.RemoveAll("/path/to/dir")
```

### Checking if Directory Exists

```go
func dirExists(fs absfs.FileSystem, path string) (bool, error) {
    info, err := fs.Stat(path)
    if err != nil {
        if os.IsNotExist(err) {
            return false, nil
        }
        return false, err
    }
    return info.IsDir(), nil
}
```

---

## io/fs Compatibility Methods

absfs includes methods for compatibility with Go's `io/fs` package, enabling interoperability with the standard library filesystem interfaces.

### ReadDir - Reading Directory Entries

The `ReadDir` method returns directory entries as `[]fs.DirEntry`, compatible with `io/fs.ReadDirFS`:

```go
// Read directory entries
entries, err := fs.ReadDir("/var/log")
if err != nil {
    return err
}

for _, entry := range entries {
    fmt.Printf("%s (dir: %v)\n", entry.Name(), entry.IsDir())

    // Get full FileInfo if needed
    info, err := entry.Info()
    if err != nil {
        continue
    }
    fmt.Printf("  Size: %d, Mode: %s\n", info.Size(), info.Mode())
}
```

**Benefits over `Readdir`:**
- Returns `fs.DirEntry` which is more efficient (doesn't always need full stat)
- Compatible with `io/fs.ReadDirFS` interface
- Sorted by filename

### ReadFile - Reading Entire Files

The `ReadFile` method reads an entire file and returns its contents, compatible with `io/fs.ReadFileFS`:

```go
// Read entire file at once
data, err := fs.ReadFile("/config/app.json")
if err != nil {
    return err
}

// Parse JSON, etc.
var config Config
if err := json.Unmarshal(data, &config); err != nil {
    return err
}
```

**Benefits over `Open` + `io.ReadAll`:**
- More concise for simple file reading
- Automatically handles file opening and closing
- Optimized allocation based on file size
- Compatible with `io/fs.ReadFileFS` interface

### Sub - Creating Subdirectory Views

The `Sub` method returns a read-only `fs.FS` rooted at a subdirectory, compatible with `io/fs.SubFS`:

```go
// Create a view of /var/log subdirectory
logFS, err := fs.Sub("/var/log")
if err != nil {
    return err
}

// Paths are now relative to /var/log
file, err := logFS.Open("app.log") // Opens /var/log/app.log
if err != nil {
    return err
}
defer file.Close()

// Use with standard library functions
err = template.ParseFS(logFS, "*.tmpl")
```

**Important Notes:**
- The returned `fs.FS` is **read-only**
- All paths must be valid `fs.FS` paths (no `..`, no absolute paths)
- For writable subdirectory access, use [basefs](https://github.com/absfs/basefs) instead

**Use Cases:**
- Passing a subdirectory to functions that accept `fs.FS`
- Working with `html/template`, `embed`, and other stdlib packages
- Creating sandboxed read-only views
- Template parsing, asset serving, etc.

### Working with io/fs Standard Library

These methods make absfs filesystems compatible with standard library functions:

```go
import (
    "html/template"
    "io/fs"
)

// Use with template.ParseFS
tmplFS, _ := fs.Sub("/templates")
tmpl, err := template.ParseFS(tmplFS, "*.html")

// Use with fs.WalkDir
err = fs.WalkDir(fs, "/data", func(path string, d fs.DirEntry, err error) error {
    if err != nil {
        return err
    }
    fmt.Println(path)
    return nil
})

// Use with fs.ReadFile
data, err := fs.ReadFile(fs, "/config.json")

// Use with fs.ReadDir
entries, err := fs.ReadDir(fs, "/logs")
```

---

## Path Handling

### Virtual-Absolute Paths

absfs treats paths starting with `/` or `\` as **virtual-absolute paths** that work consistently across all platforms:

```go
// These work identically on Unix, macOS, AND Windows:
fs.Create("/config/app.json")
fs.MkdirAll("/var/log/app", 0755)
fs.Open("/data/users.db")
```

### Cross-Platform OS Filesystem Setup

When working with the OS filesystem (osfs) across platforms, use build tags for automatic Windows drive mapping:

**Create `filesystem_windows.go`:**
```go
//go:build windows

package yourapp

import "github.com/absfs/osfs"

func NewFS(drive string) absfs.FileSystem {
    if drive == "" { drive = "C:" }
    return osfs.NewWindowsDriveMapper(osfs.NewFS(), drive)
}
```

**Create `filesystem_unix.go`:**
```go
//go:build !windows

package yourapp

import "github.com/absfs/osfs"

func NewFS(drive string) absfs.FileSystem {
    return osfs.NewFS()  // Drive ignored on Unix
}
```

**Use it everywhere:**
```go
fs := yourapp.NewFS("")
fs.Create("/config/app.json")  // Works on all platforms!
```

See [PATH_HANDLING.md](PATH_HANDLING.md) for complete details and the [cross-platform example](examples/cross-platform/).

### Platform-Specific Paths

You can also use OS-native path formats directly:

```go
// Unix/macOS
fs.Create("/home/user/file.txt")

// Windows drive letters
fs.Create("C:\\Users\\user\\file.txt")

// Windows UNC paths
fs.Open("\\\\server\\share\\file.txt")
```

### Working Directory

Each `FileSystem` instance maintains its own current working directory:

```go
// Change directory
err := fs.Chdir("/var/log")
if err != nil {
    return err
}

// Get current directory
cwd, err := fs.Getwd()
if err != nil {
    return err
}
fmt.Println("Current directory:", cwd)

// Relative paths now resolve from /var/log
file, err := fs.Open("app.log") // Opens /var/log/app.log
```

### Path Operations

```go
import "path/filepath"

// Join path components
path := filepath.Join("/var", "log", "app.log")
// Result: /var/log/app.log

// Get directory
dir := filepath.Dir("/var/log/app.log")
// Result: /var/log

// Get filename
base := filepath.Base("/var/log/app.log")
// Result: app.log

// Get extension
ext := filepath.Ext("/var/log/app.log")
// Result: .log

// Clean path
clean := filepath.Clean("/var//log/./app.log")
// Result: /var/log/app.log
```

### Best Practices

**DO:**
- Use `/` as separator in your code for portability
- Use `filepath.Join()` to construct paths
- Use absolute paths for shared filesystem instances
- Clean paths with `filepath.Clean()` when needed

**DON'T:**
- Hard-code `\` separators
- Use relative paths without considering working directory
- Assume paths have a specific format

See [PATH_HANDLING.md](PATH_HANDLING.md) for comprehensive cross-platform path documentation.

---

## Thread Safety

### ⚠️ Important: FileSystem Instances Are NOT Thread-Safe

Each `FileSystem` instance maintains a current working directory that can be changed with `Chdir()`. Concurrent access from multiple goroutines can cause race conditions.

### Safe Usage Patterns

#### Pattern 1: One FileSystem Per Goroutine (Recommended)

```go
func processFiles(files []string, filer absfs.Filer) {
    var wg sync.WaitGroup

    for _, file := range files {
        wg.Add(1)
        go func(filename string) {
            defer wg.Done()

            // Each goroutine gets its own FileSystem instance
            fs := absfs.ExtendFiler(filer)
            fs.Chdir("/work")

            // Safe: no shared state
            f, err := fs.Open(filename)
            if err != nil {
                log.Printf("Error: %v", err)
                return
            }
            defer f.Close()

            // Process file...
        }(file)
    }

    wg.Wait()
}
```

#### Pattern 2: Use Absolute Paths Only

```go
// Shared filesystem is safe if you only use absolute paths
sharedFS := osfs.NewFS()

go func() {
    // Safe: no dependency on working directory
    sharedFS.Open("/absolute/path/to/file1.txt")
}()

go func() {
    // Safe: absolute paths work independently
    sharedFS.Open("/absolute/path/to/file2.txt")
}()
```

#### Pattern 3: External Synchronization

```go
var (
    mu sync.Mutex
    sharedFS = osfs.NewFS()
)

go func() {
    mu.Lock()
    sharedFS.Chdir("/directory1")
    file, _ := sharedFS.Open("relative/file.txt")
    // ... use file ...
    mu.Unlock()
}()

go func() {
    mu.Lock()
    sharedFS.Chdir("/directory2")
    file, _ := sharedFS.Open("relative/file.txt")
    // ... use file ...
    mu.Unlock()
}()
```

### File-Level Thread Safety

Individual `File` objects returned by filesystem operations should be used by a single goroutine. If you need concurrent access to a file, coordinate with synchronization primitives.

---

## Available Filesystems

### OsFs - Operating System Filesystem

Wraps the standard `os` package:

```go
import "github.com/absfs/osfs"

fs := osfs.NewFS()
file, err := fs.Create("/tmp/test.txt")
```

### MemFs - In-Memory Filesystem

Fast, volatile storage for testing:

```go
import "github.com/absfs/memfs"

fs := memfs.NewFS()
fs.Create("/test.txt") // Stored in memory
```

### NilFs - No-Op Filesystem

Ignores all operations (useful for testing):

```go
import "github.com/absfs/nilfs"

fs := nilfs.NewFS()
fs.Create("/anything") // Does nothing, returns nil
```

### BaseFS - Chroot Filesystem

Restricts operations to a subdirectory:

```go
import "github.com/absfs/basefs"

base := osfs.NewFS()
fs := basefs.NewFS(base, "/var/app")

// All operations are relative to /var/app
fs.Create("/config.json") // Actually creates /var/app/config.json
```

### ROFS - Read-Only Filesystem

Makes any filesystem read-only:

```go
import "github.com/absfs/rofs"

base := osfs.NewFS()
fs := rofs.NewFS(base)

// Read operations work
file, _ := fs.Open("/etc/passwd")

// Write operations fail
fs.Create("/etc/test") // Returns error
```

### S3FS - S3 Storage Filesystem

Access S3-compatible storage:

```go
import "github.com/absfs/s3fs"

fs := s3fs.NewFS("bucket-name", s3Config)
fs.Create("/objects/file.txt") // Uploads to S3
```

### SftpFs - SFTP Filesystem

Access remote filesystems via SFTP:

```go
import "github.com/absfs/sftpfs"

fs := sftpfs.NewFS("user@host:22", sshConfig)
fs.Open("/remote/path/file.txt")
```

See the [README.md](README.md#file-systems) for a complete list.

---

## Composition Patterns

### Layering Filesystems

```go
// Create a read-only view of /usr with caching
osFS := osfs.NewFS()
baseFS := basefs.NewFS(osFS, "/usr")
roFS := rofs.NewFS(baseFS)

// Now roFS is a read-only view of /usr
file, err := roFS.Open("/bin/bash") // Opens /usr/bin/bash read-only
```

### Copy-on-Write Pattern

```go
import "github.com/absfs/cowfs"

// Read from base, write to overlay
base := osfs.NewFS()
overlay := memfs.NewFS()
fs := cowfs.NewFS(base, overlay)

// Reads come from base
fs.Open("/etc/config") // Reads from base filesystem

// Writes go to overlay
fs.Create("/etc/config") // Writes to overlay, base unchanged
```

### Cache-on-Read Pattern

```go
import "github.com/absfs/corfs"

// Cache remote filesystem locally
remote := s3fs.NewFS("bucket", config)
cache := memfs.NewFS()
fs := corfs.NewFS(remote, cache)

// First read fetches from remote and caches
fs.Open("/data.json") // Slow: fetches from S3

// Second read comes from cache
fs.Open("/data.json") // Fast: from memory
```

### Testing with Mock Filesystem

```go
func TestMyCode(t *testing.T) {
    // Use memory filesystem for tests
    fs := memfs.NewFS()

    // Set up test data
    fs.MkdirAll("/test/data", 0755)
    f, _ := fs.Create("/test/data/input.txt")
    f.Write([]byte("test data"))
    f.Close()

    // Test your code
    result := myFunction(fs)

    // Verify results
    output, _ := fs.Open("/test/data/output.txt")
    // ... assertions ...
}
```

---

## Testing with absfs

### Unit Testing with Memory Filesystem

```go
func processFile(fs absfs.FileSystem, path string) error {
    file, err := fs.Open(path)
    if err != nil {
        return err
    }
    defer file.Close()

    // Process file...
    return nil
}

func TestProcessFile(t *testing.T) {
    // Create test filesystem
    fs := memfs.NewFS()

    // Set up test data
    f, err := fs.Create("/test.txt")
    if err != nil {
        t.Fatal(err)
    }
    f.WriteString("test data")
    f.Close()

    // Test the function
    err = processFile(fs, "/test.txt")
    if err != nil {
        t.Errorf("processFile failed: %v", err)
    }
}
```

### Table-Driven Tests

```go
func TestFileOperations(t *testing.T) {
    tests := []struct {
        name    string
        setup   func(absfs.FileSystem)
        test    func(absfs.FileSystem) error
        wantErr bool
    }{
        {
            name: "create new file",
            setup: func(fs absfs.FileSystem) {
                fs.MkdirAll("/test", 0755)
            },
            test: func(fs absfs.FileSystem) error {
                _, err := fs.Create("/test/new.txt")
                return err
            },
            wantErr: false,
        },
        {
            name: "open missing file",
            setup: func(fs absfs.FileSystem) {},
            test: func(fs absfs.FileSystem) error {
                _, err := fs.Open("/missing.txt")
                return err
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            fs := memfs.NewFS()
            tt.setup(fs)
            err := tt.test(fs)
            if (err != nil) != tt.wantErr {
                t.Errorf("got error = %v, wantErr = %v", err, tt.wantErr)
            }
        })
    }
}
```

### Testing Error Conditions

```go
func TestErrorHandling(t *testing.T) {
    fs := memfs.NewFS()

    // Test file not found
    _, err := fs.Open("/nonexistent.txt")
    if !errors.Is(err, os.ErrNotExist) {
        t.Errorf("expected ErrNotExist, got %v", err)
    }

    // Test directory not empty
    fs.MkdirAll("/dir/subdir", 0755)
    err = fs.Remove("/dir")
    if err == nil {
        t.Error("expected error removing non-empty directory")
    }

    // Test create exclusive
    fs.Create("/exists.txt")
    _, err = fs.OpenFile("/exists.txt", absfs.O_CREATE|absfs.O_EXCL, 0644)
    if !errors.Is(err, os.ErrExist) {
        t.Errorf("expected ErrExist, got %v", err)
    }
}
```

### Dependency Injection

```go
type DataStore struct {
    fs absfs.FileSystem
}

func NewDataStore(fs absfs.FileSystem) *DataStore {
    return &DataStore{fs: fs}
}

func (ds *DataStore) SaveData(name string, data []byte) error {
    file, err := ds.fs.Create(name)
    if err != nil {
        return err
    }
    defer file.Close()

    _, err = file.Write(data)
    return err
}

// In tests
func TestDataStore(t *testing.T) {
    fs := memfs.NewFS()
    store := NewDataStore(fs)

    err := store.SaveData("/data.json", []byte(`{"test": true}`))
    if err != nil {
        t.Fatal(err)
    }

    // Verify file was created
    info, err := fs.Stat("/data.json")
    if err != nil {
        t.Fatal(err)
    }
    if info.Size() == 0 {
        t.Error("file is empty")
    }
}
```

---

## Best Practices

### 1. Accept FileSystem Interfaces

```go
// Good: Accepts any filesystem
func processFiles(fs absfs.FileSystem) error {
    // Works with OS, memory, S3, etc.
}

// Bad: Hard-coded to OS filesystem
func processFiles() error {
    file, err := os.Open("/path/file.txt")
    // Only works with real filesystem
}
```

### 2. Use Absolute Paths for Shared Instances

```go
// Good: Predictable behavior
sharedFS.Open("/var/log/app.log")

// Risky: Depends on working directory
sharedFS.Open("app.log") // Where is this?
```

### 3. Always Close Files

```go
// Good: Guaranteed cleanup
file, err := fs.Open("/path/file.txt")
if err != nil {
    return err
}
defer file.Close()

// Bad: Resource leak if error occurs
file, err := fs.Open("/path/file.txt")
file.Close() // Never called if Open fails
```

### 4. Check Errors

```go
// Good: Handle errors properly
file, err := fs.Create("/path/file.txt")
if err != nil {
    return fmt.Errorf("create file: %w", err)
}
defer file.Close()

_, err = file.Write(data)
if err != nil {
    return fmt.Errorf("write file: %w", err)
}

// Bad: Ignoring errors
file, _ := fs.Create("/path/file.txt")
file.Write(data)
```

### 5. Use MkdirAll for Directory Creation

```go
// Good: Creates parent directories
fs.MkdirAll("/var/app/data/logs", 0755)

// Bad: Fails if parents don't exist
fs.Mkdir("/var/app/data/logs", 0755)
```

### 6. Sync Important Data

```go
file, err := fs.Create("/important.dat")
if err != nil {
    return err
}
defer file.Close()

_, err = file.Write(criticalData)
if err != nil {
    return err
}

// Ensure data is written to storage
if err := file.Sync(); err != nil {
    return err
}
```

### 7. Use filepath Package for Path Operations

```go
import "path/filepath"

// Good: Cross-platform
path := filepath.Join(base, "subdir", "file.txt")
dir := filepath.Dir(path)
name := filepath.Base(path)

// Bad: Platform-specific
path := base + "/" + "subdir" + "/" + "file.txt" // Fails on Windows
```

### 8. Test with Multiple Filesystems

```go
func TestWithDifferentFilesystems(t *testing.T) {
    filesystems := []struct {
        name string
        fs   absfs.FileSystem
    }{
        {"memory", memfs.NewFS()},
        {"nil", nilfs.NewFS()},
        // Could add osfs in integration tests
    }

    for _, tt := range filesystems {
        t.Run(tt.name, func(t *testing.T) {
            testFileOperations(t, tt.fs)
        })
    }
}
```

---

## Common Patterns and Recipes

### Reading Entire File

```go
func readFile(fs absfs.FileSystem, path string) ([]byte, error) {
    file, err := fs.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    return io.ReadAll(file)
}
```

### Writing Entire File Atomically

```go
func writeFileAtomic(fs absfs.FileSystem, path string, data []byte) error {
    // Write to temporary file
    tmpPath := path + ".tmp"
    file, err := fs.Create(tmpPath)
    if err != nil {
        return err
    }

    _, err = file.Write(data)
    if err != nil {
        file.Close()
        fs.Remove(tmpPath)
        return err
    }

    // Sync to disk
    if err := file.Sync(); err != nil {
        file.Close()
        fs.Remove(tmpPath)
        return err
    }
    file.Close()

    // Atomic rename
    return fs.Rename(tmpPath, path)
}
```

### Checking if File Exists

```go
func fileExists(fs absfs.FileSystem, path string) (bool, error) {
    _, err := fs.Stat(path)
    if err == nil {
        return true, nil
    }
    if os.IsNotExist(err) {
        return false, nil
    }
    return false, err
}
```

### Ensuring Directory Exists

```go
func ensureDir(fs absfs.FileSystem, path string) error {
    info, err := fs.Stat(path)
    if err == nil {
        if !info.IsDir() {
            return fmt.Errorf("%s exists but is not a directory", path)
        }
        return nil
    }

    if os.IsNotExist(err) {
        return fs.MkdirAll(path, 0755)
    }

    return err
}
```

---

## Additional Resources

- [README.md](README.md) - Package overview and quick start
- [IMPLEMENTER_GUIDE.md](IMPLEMENTER_GUIDE.md) - How to implement custom filesystems
- [PATH_HANDLING.md](PATH_HANDLING.md) - Comprehensive cross-platform path documentation
- [ARCHITECTURE.md](ARCHITECTURE.md) - Design decisions and internal architecture
- [SECURITY.md](SECURITY.md) - Security considerations
- [GoDoc](https://pkg.go.dev/github.com/absfs/absfs) - API reference

---

## Getting Help

- Open an issue: https://github.com/absfs/absfs/issues
- Read the documentation files linked above
- Check existing filesystem implementations for examples

---

## Examples from the Ecosystem

Browse the source code of existing implementations to learn patterns:

- [osfs](https://github.com/absfs/osfs) - OS filesystem wrapper
- [memfs](https://github.com/absfs/memfs) - In-memory filesystem
- [basefs](https://github.com/absfs/basefs) - Chroot filesystem
- [rofs](https://github.com/absfs/rofs) - Read-only wrapper
- [s3fs](https://github.com/absfs/s3fs) - S3 storage filesystem

Each implementation demonstrates different approaches and patterns you can apply in your own code.
