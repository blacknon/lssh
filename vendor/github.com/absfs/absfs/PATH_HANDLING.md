# Path Handling in absfs

**Audience**: All users of absfs - covers cross-platform path semantics and conventions.

**Quick Start:** All absfs filesystems use Unix-style forward slash paths (`/`) on all platforms. On Windows, drive letters are represented as `/c/`, `/d/`, etc.

---

## Universal Unix-Style Paths

All absfs filesystems use Unix-style paths for consistency and composability:

```go
import (
    "github.com/absfs/memfs"
    "github.com/absfs/osfs"
)

// Virtual filesystem - Unix paths everywhere
vfs, _ := memfs.NewFS()
vfs.Create("/config/app.json")
vfs.MkdirAll("/var/log/app", 0755)

// OS filesystem - also Unix paths
fs, _ := osfs.NewFS()
fs.Create("/home/user/file.txt")       // Unix: /home/user/file.txt
fs.Create("/c/Users/test/file.txt")    // Windows: C:\Users\test\file.txt
```

This means:

- **Paths from one absfs filesystem work in another** - composability
- **`absfs.Separator` is always `/`** on all platforms
- **`absfs.ListSeparator` is always `:`** on all platforms
- **`Getwd()`, `File.Name()`, etc. return Unix-style paths**

---

## Windows Drive Letter Convention

On Windows, drive letters are represented using Git Bash / MSYS2 style:

| Native Windows Path | absfs Unix-Style |
|---------------------|------------------|
| `C:\Users\foo` | `/c/Users/foo` |
| `D:\Data\bar` | `/d/Data/bar` |
| `\\server\share\path` | `//server/share/path` |

### Current Drive

When you create an `osfs.FileSystem`, the current drive is derived from the working directory:

```go
// On Windows, if cwd is C:\Users\test:
fs, _ := osfs.NewFS()
cwd, _ := fs.Getwd()
// cwd = "/c/Users/test"

// Paths without a drive prefix use the current drive
fs.Open("/foo/bar")  // Opens C:\foo\bar
```

### Changing Drives

Use `Chdir` to change to a different drive:

```go
fs.Chdir("/d/Data")   // Changes to D:\Data
fs.Chdir("/c/")       // Changes to C:\ root
```

---

## Path Helpers (osfs package)

The `osfs` package provides helpers for converting between Unix-style absfs paths and native OS paths:

### ToNative / FromNative

Convert between path formats when interacting with native APIs:

```go
import "github.com/absfs/osfs"

// Convert absfs path to native for external tools
nativePath := osfs.ToNative("/c/Users/foo/file.txt")
// Windows: "C:\Users\foo\file.txt"
// Unix:    "/c/Users/foo/file.txt" (unchanged)

// Convert native path to absfs format
absPath := osfs.FromNative(`C:\Users\foo\file.txt`)
// Windows: "/c/Users/foo/file.txt"
// Unix:    "C:\Users\foo\file.txt" (unchanged)
```

### Drive Letter Manipulation

```go
// Extract drive letter
drive := osfs.GetDrive("/c/Users/foo")  // "c"

// Split into drive and path
drive, rest := osfs.SplitDrive("/c/Users/foo")  // "c", "/Users/foo"

// Set/change drive
newPath := osfs.SetDrive("/c/foo", "d")  // "/d/foo"

// Remove drive prefix
stripped := osfs.StripDrive("/c/foo")  // "/foo"

// Combine drive with path
joined := osfs.JoinDrive("c", "/Users/foo")  // "/c/Users/foo"
```

### UNC Path Handling

```go
// Check if path is UNC
isUnc := osfs.IsUNC("//server/share/path")  // true

// Split UNC path
server, share, rest := osfs.SplitUNC("//server/share/foo/bar")
// server="server", share="share", rest="/foo/bar"

// Create UNC path
unc := osfs.JoinUNC("server", "share", "/foo")  // "//server/share/foo"
```

### Path Validation

```go
// Check for Windows-invalid paths
err := osfs.ValidatePath("/c/CON.txt")  // Error: reserved name

// Check reserved names
reserved := osfs.IsReservedName("NUL")  // true on Windows, false on Unix
```

---

## Cross-Platform Best Practices

### Use Unix-Style Paths in Application Code

```go
// Good - works everywhere
fs.Create("/data/config.json")
fs.MkdirAll("/var/log/app", 0755)

// Avoid - Windows-specific
fs.Create("C:\\data\\config.json")  // Only works on Windows
```

### Use path.Join for Virtual Paths

```go
import "path"  // Not path/filepath!

// Good - always produces forward slashes
configPath := path.Join("/data", "config", "app.json")
// Result: "/data/config/app.json"

// Avoid for virtual paths - may produce backslashes on Windows
configPath := filepath.Join("/data", "config", "app.json")
// Windows: "\data\config\app.json" (wrong for absfs)
```

### Convert When Calling External APIs

```go
import (
    "os/exec"
    "github.com/absfs/osfs"
)

// When calling native tools that expect native paths:
absPath := "/c/Users/foo/script.sh"
nativePath := osfs.ToNative(absPath)
cmd := exec.Command(nativePath)

// When receiving paths from native sources:
nativePath := os.Getenv("HOME")
absPath := osfs.FromNative(nativePath)
```

---

## Composability Example

The key benefit of consistent Unix-style paths is composability:

```go
import (
    "github.com/absfs/memfs"
    "github.com/absfs/osfs"
    "github.com/absfs/unionfs"
)

// Create overlay filesystem
overlay, _ := memfs.NewFS()
base, _ := osfs.NewFS()

ufs := unionfs.New(
    unionfs.WithWritableLayer(overlay),
    unionfs.WithReadOnlyLayer(base),
)

// Paths work consistently regardless of underlying filesystem
ufs.Open("/etc/config.yml")     // Works
ufs.Create("/tmp/cache.dat")    // Works
ufs.MkdirAll("/var/log", 0755)  // Works

// On Windows with osfs as base:
// /etc/config.yml maps to C:\etc\config.yml
// The unionfs doesn't need to know about Windows paths
```

---

## Platform Behavior Summary

| Operation | Unix | Windows |
|-----------|------|---------|
| `absfs.Separator` | `/` | `/` |
| `absfs.ListSeparator` | `:` | `:` |
| `Getwd()` | `/home/user` | `/c/Users/user` |
| `TempDir()` | `/tmp` | `/c/Users/user/AppData/Local/Temp` |
| `File.Name()` | `/path/to/file` | `/c/path/to/file` |
| `Open("/foo")` | Opens `/foo` | Opens `C:\foo` (current drive) |
| `Open("/c/foo")` | Opens `/c/foo` | Opens `C:\foo` |

---

## Migration from WindowsDriveMapper

If you were using the old `WindowsDriveMapper` pattern, migration is simple - just remove it:

**Before (old pattern):**
```go
//go:build windows

func NewFS(drive string) absfs.FileSystem {
    return osfs.NewWindowsDriveMapper(osfs.NewFS(), drive)
}
```

**After (new pattern):**
```go
// Works on all platforms, no build tags needed
func NewFS() (absfs.FileSystem, error) {
    return osfs.NewFS()
}
```

The osfs filesystem now handles drive letters internally using the `/c/path` convention.

---

## Summary

- **All absfs filesystems use Unix-style `/` paths**
- **Drive letters on Windows: `/c/`, `/d/`, etc.**
- **UNC paths: `//server/share/path`**
- **Use `path.Join` (not `filepath.Join`) for virtual paths**
- **Use `osfs.ToNative()` / `osfs.FromNative()` for native API interop**
- **Paths are composable across different filesystem implementations**

For complete API reference, see the [osfs package documentation](https://pkg.go.dev/github.com/absfs/osfs).
