# Detailed Prompt: Adding WindowsDriveMapper to osfs Package

## Overview
Add a `WindowsDriveMapper` wrapper to the osfs package that translates virtual-absolute paths (like `/config/app.json`) to OS-absolute paths on Windows (like `C:\config\app.json`). This makes cross-platform code more intuitive when working with OS filesystems on Windows.

## Rationale
- The absfs package already handles virtual-absolute paths correctly with the `isVirtualAbs()` helper
- However, when using osfs (OS filesystem), Windows users may want `/path` to map to `C:\path` for OS-level operations
- This is an **optional** wrapper for users who need OS compatibility rather than virtual filesystem semantics
- Lives in osfs because it's OS-specific, not universally applicable to all absfs implementations

## Implementation Details

### File 1: `drivemapper_windows.go`

Create a Windows-specific file with build tag:

```go
// +build windows

package osfs

import (
	"os"
	"path/filepath"
	"time"

	"github.com/absfs/absfs"
)

// WindowsDriveMapper wraps an absfs.FileSystem and translates virtual-absolute
// paths to OS-absolute paths on Windows by prepending a drive letter.
//
// This is useful when you want Unix-style absolute paths like "/config/app.json"
// to map to Windows paths like "C:\config\app.json" for OS filesystem operations.
//
// Path translation rules:
//   - Virtual-absolute (starts with / or \) → Prepend drive letter
//   - OS-absolute (has drive letter or UNC) → Pass through unchanged
//   - Relative paths → Pass through unchanged
//
// Example:
//   fs := osfs.NewFS()
//   mapped := osfs.NewWindowsDriveMapper(fs, "C:")
//
//   mapped.Create("/config/app.json")      // → C:\config\app.json
//   mapped.Open("C:\\Windows\\file.txt")   // → C:\Windows\file.txt (unchanged)
//   mapped.MkdirAll("/var/log", 0755)      // → C:\var\log
type WindowsDriveMapper struct {
	base  absfs.FileSystem
	drive string
}

// NewWindowsDriveMapper creates a new WindowsDriveMapper that wraps the given
// FileSystem and translates virtual-absolute paths to use the specified drive.
//
// If drive is empty, defaults to "C:". Drive should be in the format "C:" or "D:".
func NewWindowsDriveMapper(base absfs.FileSystem, drive string) absfs.FileSystem {
	if drive == "" {
		drive = "C:"
	}
	return &WindowsDriveMapper{
		base:  base,
		drive: drive,
	}
}

// translatePath converts virtual-absolute paths to OS-absolute paths.
// OS-absolute and relative paths pass through unchanged.
func (w *WindowsDriveMapper) translatePath(path string) string {
	// Already OS-absolute (has drive letter or UNC) - no translation needed
	if filepath.IsAbs(path) {
		return path
	}

	// Virtual-absolute (starts with / or \) - add drive letter
	if len(path) > 0 && (path[0] == '/' || path[0] == '\\') {
		return filepath.Join(w.drive+"\\", path)
	}

	// Relative path - no translation
	return path
}

// Implement all absfs.FileSystem interface methods with path translation

func (w *WindowsDriveMapper) OpenFile(name string, flag int, perm os.FileMode) (absfs.File, error) {
	return w.base.OpenFile(w.translatePath(name), flag, perm)
}

func (w *WindowsDriveMapper) Mkdir(name string, perm os.FileMode) error {
	return w.base.Mkdir(w.translatePath(name), perm)
}

func (w *WindowsDriveMapper) Remove(name string) error {
	return w.base.Remove(w.translatePath(name))
}

func (w *WindowsDriveMapper) Rename(oldpath, newpath string) error {
	return w.base.Rename(w.translatePath(oldpath), w.translatePath(newpath))
}

func (w *WindowsDriveMapper) Stat(name string) (os.FileInfo, error) {
	return w.base.Stat(w.translatePath(name))
}

func (w *WindowsDriveMapper) Chmod(name string, mode os.FileMode) error {
	return w.base.Chmod(w.translatePath(name), mode)
}

func (w *WindowsDriveMapper) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return w.base.Chtimes(w.translatePath(name), atime, mtime)
}

func (w *WindowsDriveMapper) Chown(name string, uid, gid int) error {
	return w.base.Chown(w.translatePath(name), uid, gid)
}

// Extended FileSystem methods

func (w *WindowsDriveMapper) Open(name string) (absfs.File, error) {
	return w.base.Open(w.translatePath(name))
}

func (w *WindowsDriveMapper) Create(name string) (absfs.File, error) {
	return w.base.Create(w.translatePath(name))
}

func (w *WindowsDriveMapper) MkdirAll(path string, perm os.FileMode) error {
	return w.base.MkdirAll(w.translatePath(path), perm)
}

func (w *WindowsDriveMapper) RemoveAll(path string) error {
	return w.base.RemoveAll(w.translatePath(path))
}

func (w *WindowsDriveMapper) Truncate(name string, size int64) error {
	return w.base.Truncate(w.translatePath(name), size)
}

func (w *WindowsDriveMapper) Chdir(dir string) error {
	return w.base.Chdir(w.translatePath(dir))
}

// Pass-through methods (no path translation needed)

func (w *WindowsDriveMapper) Separator() uint8 {
	return w.base.Separator()
}

func (w *WindowsDriveMapper) ListSeparator() uint8 {
	return w.base.ListSeparator()
}

func (w *WindowsDriveMapper) Getwd() (dir string, err error) {
	return w.base.Getwd()
}

func (w *WindowsDriveMapper) TempDir() string {
	return w.base.TempDir()
}
```

### File 2: `drivemapper_unix.go`

Create a no-op Unix implementation:

```go
// +build !windows

package osfs

import "github.com/absfs/absfs"

// NewWindowsDriveMapper on non-Windows platforms simply returns the base FileSystem
// unchanged, since virtual-absolute paths already work correctly on Unix-like systems.
//
// This no-op implementation ensures code using NewWindowsDriveMapper compiles and
// runs correctly on all platforms without conditional compilation in user code.
//
// Example:
//   // This code works on all platforms:
//   fs := osfs.NewWindowsDriveMapper(osfs.NewFS(), "C:")
//   fs.Create("/config/app.json")
//   // On Unix/macOS: creates /config/app.json
//   // On Windows: creates C:\config\app.json
func NewWindowsDriveMapper(base absfs.FileSystem, drive string) absfs.FileSystem {
	return base
}
```

### File 3: `drivemapper_test.go`

Create Windows-specific tests:

```go
// +build windows

package osfs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/absfs/absfs"
)

func TestWindowsDriveMapperTranslatePath(t *testing.T) {
	base := NewFS()
	mapper := NewWindowsDriveMapper(base, "C:").(*WindowsDriveMapper)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Virtual-absolute with forward slash",
			input:    "/config/app.json",
			expected: "C:\\config\\app.json",
		},
		{
			name:     "Virtual-absolute with backslash",
			input:    "\\var\\log\\app.log",
			expected: "C:\\var\\log\\app.log",
		},
		{
			name:     "OS-absolute with drive letter",
			input:    "C:\\Windows\\System32\\file.txt",
			expected: "C:\\Windows\\System32\\file.txt",
		},
		{
			name:     "OS-absolute with different drive",
			input:    "D:\\Data\\file.txt",
			expected: "D:\\Data\\file.txt",
		},
		{
			name:     "UNC path",
			input:    "\\\\server\\share\\file.txt",
			expected: "\\\\server\\share\\file.txt",
		},
		{
			name:     "Relative path",
			input:    "relative\\path\\file.txt",
			expected: "relative\\path\\file.txt",
		},
		{
			name:     "Root path",
			input:    "/",
			expected: "C:\\",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.translatePath(tt.input)
			expected := filepath.Clean(tt.expected)
			result = filepath.Clean(result)

			if result != expected {
				t.Errorf("translatePath(%q) = %q, want %q", tt.input, result, expected)
			}
		})
	}
}

func TestWindowsDriveMapperDefaultDrive(t *testing.T) {
	base := NewFS()
	mapper := NewWindowsDriveMapper(base, "").(*WindowsDriveMapper)

	if mapper.drive != "C:" {
		t.Errorf("Expected default drive C:, got %s", mapper.drive)
	}
}

func TestWindowsDriveMapperCustomDrive(t *testing.T) {
	base := NewFS()
	mapper := NewWindowsDriveMapper(base, "D:").(*WindowsDriveMapper)

	if mapper.drive != "D:" {
		t.Errorf("Expected drive D:, got %s", mapper.drive)
	}

	result := mapper.translatePath("/data/file.txt")
	expected := filepath.Clean("D:\\data\\file.txt")
	result = filepath.Clean(result)

	if result != expected {
		t.Errorf("translatePath with D: drive = %q, want %q", result, expected)
	}
}

func TestWindowsDriveMapperIntegration(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	base := NewFS()
	mapper := NewWindowsDriveMapper(base, "C:")

	// Test that OS-absolute paths work unchanged
	testFile := filepath.Join(tempDir, "test.txt")
	f, err := mapper.Create(testFile)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	f.Close()

	// Verify file was created
	_, err = mapper.Stat(testFile)
	if err != nil {
		t.Errorf("Stat failed: %v", err)
	}

	// Clean up
	err = mapper.Remove(testFile)
	if err != nil {
		t.Errorf("Remove failed: %v", err)
	}
}

func TestWindowsDriveMapperPassThroughMethods(t *testing.T) {
	base := NewFS()
	mapper := NewWindowsDriveMapper(base, "C:")

	// Test methods that should pass through unchanged
	if sep := mapper.Separator(); sep != base.Separator() {
		t.Errorf("Separator() = %c, want %c", sep, base.Separator())
	}

	if listSep := mapper.ListSeparator(); listSep != base.ListSeparator() {
		t.Errorf("ListSeparator() = %c, want %c", listSep, base.ListSeparator())
	}

	if tempDir := mapper.TempDir(); tempDir != base.TempDir() {
		t.Errorf("TempDir() = %s, want %s", tempDir, base.TempDir())
	}
}

func TestWindowsDriveMapperAllMethods(t *testing.T) {
	tempDir := t.TempDir()
	base := NewFS()
	mapper := NewWindowsDriveMapper(base, "C:")

	// Test various FileSystem methods with OS-absolute paths
	testPath := filepath.Join(tempDir, "test")

	// Mkdir
	err := mapper.Mkdir(testPath, 0755)
	if err != nil {
		t.Errorf("Mkdir failed: %v", err)
	}

	// Stat
	info, err := mapper.Stat(testPath)
	if err != nil {
		t.Errorf("Stat failed: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected directory")
	}

	// Create file
	filePath := filepath.Join(testPath, "file.txt")
	f, err := mapper.Create(filePath)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	f.Close()

	// Chmod
	err = mapper.Chmod(filePath, 0644)
	if err != nil {
		t.Errorf("Chmod failed: %v", err)
	}

	// Chtimes
	now := time.Now()
	err = mapper.Chtimes(filePath, now, now)
	if err != nil {
		t.Errorf("Chtimes failed: %v", err)
	}

	// Rename
	newPath := filepath.Join(testPath, "renamed.txt")
	err = mapper.Rename(filePath, newPath)
	if err != nil {
		t.Errorf("Rename failed: %v", err)
	}

	// Truncate
	err = mapper.Truncate(newPath, 100)
	if err != nil {
		t.Errorf("Truncate failed: %v", err)
	}

	// RemoveAll
	err = mapper.RemoveAll(testPath)
	if err != nil {
		t.Errorf("RemoveAll failed: %v", err)
	}
}
```

## Documentation Updates

### Update osfs README.md

Add a section explaining the WindowsDriveMapper:

```markdown
## Windows Drive Mapping

On Windows, the `WindowsDriveMapper` wrapper provides intuitive path translation for cross-platform code:

```go
import "github.com/absfs/osfs"

// Create a mapper that translates virtual-absolute paths
fs := osfs.NewWindowsDriveMapper(osfs.NewFS(), "C:")

// Unix-style paths automatically map to Windows paths
fs.Create("/config/app.json")      // → C:\config\app.json
fs.MkdirAll("/var/log/app", 0755)  // → C:\var\log\app

// OS-absolute paths pass through unchanged
fs.Open("C:\\Windows\\file.txt")   // → C:\Windows\file.txt
fs.Open("D:\\Data\\file.txt")      // → D:\Data\file.txt

// UNC paths work correctly
fs.Open("\\\\server\\share\\file") // → \\server\share\file

// On Unix/macOS, the mapper is a no-op
fs.Create("/config/app.json")      // → /config/app.json
```

**When to use WindowsDriveMapper:**
- Writing cross-platform CLI tools that work with OS filesystems
- Porting Unix-based tools to Windows
- When you want `/path` semantics to map to `C:\path` on Windows

**When NOT to use WindowsDriveMapper:**
- Virtual/in-memory filesystems (use base absfs)
- When you need full control over Windows drive letters in your code
- Testing with mock filesystems

See [absfs PATH_HANDLING.md](https://github.com/absfs/absfs/blob/main/PATH_HANDLING.md) for more details on cross-platform path handling.
```

## Example Usage

Add an example file `example_drivemapper_test.go`:

```go
// +build windows

package osfs_test

import (
	"fmt"
	"log"

	"github.com/absfs/osfs"
)

func ExampleNewWindowsDriveMapper() {
	// Create an OS filesystem with drive mapping
	fs := osfs.NewWindowsDriveMapper(osfs.NewFS(), "C:")

	// Unix-style paths work intuitively on Windows
	f, err := fs.Create("/tmp/config.json")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// This created C:\tmp\config.json on Windows
	// and /tmp/config.json on Unix/macOS

	fmt.Println("Config file created")
	// Output: Config file created
}

func ExampleNewWindowsDriveMapper_customDrive() {
	// Use a different drive letter
	fs := osfs.NewWindowsDriveMapper(osfs.NewFS(), "D:")

	// Paths map to D: drive instead
	err := fs.MkdirAll("/data/logs", 0755)
	if err != nil {
		log.Fatal(err)
	}

	// This created D:\data\logs on Windows
	// and /data/logs on Unix/macOS

	fmt.Println("Directory created")
	// Output: Directory created
}
```

## Testing Strategy

1. **Unit tests** (drivemapper_test.go):
   - Test path translation logic with various inputs
   - Test default drive behavior
   - Test custom drive configuration
   - Test all FileSystem methods

2. **Integration tests**:
   - Create/read/write actual files through the mapper
   - Verify correct OS-level paths are created
   - Test with different drive letters

3. **Cross-platform verification**:
   - Verify Unix stub compiles and works correctly
   - Ensure no build errors on all platforms

## Summary

This implementation:
- ✅ Lives in osfs (OS-specific, not universal)
- ✅ Named "DriveMapper" (more accurate than "PathMapper")
- ✅ Optional wrapper for OS filesystem operations
- ✅ Maintains cross-platform compatibility
- ✅ Well-tested and documented
- ✅ Uses build tags for platform-specific code
- ✅ Follows absfs interface conventions
- ✅ Provides clear examples and documentation

Users who need virtual filesystem semantics use base absfs. Users who need OS compatibility on Windows use osfs with WindowsDriveMapper.
