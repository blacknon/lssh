// Package absfs provides abstract filesystem interfaces.
// This package is deprecated and exists only for backward compatibility.
// Use github.com/absfs/absfs instead.
package absfs

import (
	"io/fs"

	"github.com/absfs/absfs"
)

// FileSystem is an alias for absfs.FileSystem
// Deprecated: Use github.com/absfs/absfs.FileSystem instead.
type FileSystem = absfs.FileSystem

// File is an alias for absfs.File
// Deprecated: Use github.com/absfs/absfs.File instead.
type File = absfs.File

// Filer is an alias for absfs.Filer
// Deprecated: Use github.com/absfs/absfs.Filer instead.
type Filer = absfs.Filer

// FilerToFS creates an fs.FS from any Filer, rooted at the given directory.
// Deprecated: Use github.com/absfs/absfs.FilerToFS instead.
func FilerToFS(filer Filer, root string) (fs.FS, error) {
	return absfs.FilerToFS(filer, root)
}
