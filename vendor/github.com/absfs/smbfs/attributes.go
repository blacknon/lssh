package smbfs

import (
	"io/fs"
)

// Windows file attribute flags as defined in MS-FSCC.
const (
	// FILE_ATTRIBUTE_READONLY indicates the file is read-only.
	FILE_ATTRIBUTE_READONLY = 0x00000001

	// FILE_ATTRIBUTE_HIDDEN indicates the file is hidden.
	FILE_ATTRIBUTE_HIDDEN = 0x00000002

	// FILE_ATTRIBUTE_SYSTEM indicates the file is a system file.
	FILE_ATTRIBUTE_SYSTEM = 0x00000004

	// FILE_ATTRIBUTE_DIRECTORY indicates the file is a directory.
	FILE_ATTRIBUTE_DIRECTORY = 0x00000010

	// FILE_ATTRIBUTE_ARCHIVE indicates the file should be archived.
	FILE_ATTRIBUTE_ARCHIVE = 0x00000020

	// FILE_ATTRIBUTE_DEVICE indicates the file is a device.
	FILE_ATTRIBUTE_DEVICE = 0x00000040

	// FILE_ATTRIBUTE_NORMAL indicates the file has no other attributes set.
	FILE_ATTRIBUTE_NORMAL = 0x00000080

	// FILE_ATTRIBUTE_TEMPORARY indicates the file is temporary.
	FILE_ATTRIBUTE_TEMPORARY = 0x00000100

	// FILE_ATTRIBUTE_SPARSE_FILE indicates the file is a sparse file.
	FILE_ATTRIBUTE_SPARSE_FILE = 0x00000200

	// FILE_ATTRIBUTE_REPARSE_POINT indicates the file is a reparse point (symlink/junction).
	FILE_ATTRIBUTE_REPARSE_POINT = 0x00000400

	// FILE_ATTRIBUTE_COMPRESSED indicates the file is compressed.
	FILE_ATTRIBUTE_COMPRESSED = 0x00000800

	// FILE_ATTRIBUTE_OFFLINE indicates the file data is offline.
	FILE_ATTRIBUTE_OFFLINE = 0x00001000

	// FILE_ATTRIBUTE_NOT_CONTENT_INDEXED indicates the file should not be indexed.
	FILE_ATTRIBUTE_NOT_CONTENT_INDEXED = 0x00002000

	// FILE_ATTRIBUTE_ENCRYPTED indicates the file is encrypted.
	FILE_ATTRIBUTE_ENCRYPTED = 0x00004000
)

// WindowsAttributes represents Windows-specific file attributes.
type WindowsAttributes struct {
	attrs uint32
}

// NewWindowsAttributes creates a new WindowsAttributes from a uint32 value.
func NewWindowsAttributes(attrs uint32) *WindowsAttributes {
	return &WindowsAttributes{attrs: attrs}
}

// Attributes returns the raw attribute value.
func (wa *WindowsAttributes) Attributes() uint32 {
	return wa.attrs
}

// IsHidden returns true if the file has the hidden attribute.
func (wa *WindowsAttributes) IsHidden() bool {
	return wa.attrs&FILE_ATTRIBUTE_HIDDEN != 0
}

// IsSystem returns true if the file has the system attribute.
func (wa *WindowsAttributes) IsSystem() bool {
	return wa.attrs&FILE_ATTRIBUTE_SYSTEM != 0
}

// IsReadOnly returns true if the file has the read-only attribute.
func (wa *WindowsAttributes) IsReadOnly() bool {
	return wa.attrs&FILE_ATTRIBUTE_READONLY != 0
}

// IsArchive returns true if the file has the archive attribute.
func (wa *WindowsAttributes) IsArchive() bool {
	return wa.attrs&FILE_ATTRIBUTE_ARCHIVE != 0
}

// IsTemporary returns true if the file has the temporary attribute.
func (wa *WindowsAttributes) IsTemporary() bool {
	return wa.attrs&FILE_ATTRIBUTE_TEMPORARY != 0
}

// IsSparse returns true if the file is a sparse file.
func (wa *WindowsAttributes) IsSparse() bool {
	return wa.attrs&FILE_ATTRIBUTE_SPARSE_FILE != 0
}

// IsReparsePoint returns true if the file is a reparse point (symlink/junction).
func (wa *WindowsAttributes) IsReparsePoint() bool {
	return wa.attrs&FILE_ATTRIBUTE_REPARSE_POINT != 0
}

// IsCompressed returns true if the file is compressed.
func (wa *WindowsAttributes) IsCompressed() bool {
	return wa.attrs&FILE_ATTRIBUTE_COMPRESSED != 0
}

// IsOffline returns true if the file data is offline.
func (wa *WindowsAttributes) IsOffline() bool {
	return wa.attrs&FILE_ATTRIBUTE_OFFLINE != 0
}

// IsEncrypted returns true if the file is encrypted.
func (wa *WindowsAttributes) IsEncrypted() bool {
	return wa.attrs&FILE_ATTRIBUTE_ENCRYPTED != 0
}

// SetHidden sets or clears the hidden attribute.
func (wa *WindowsAttributes) SetHidden(hidden bool) {
	if hidden {
		wa.attrs |= FILE_ATTRIBUTE_HIDDEN
	} else {
		wa.attrs &^= FILE_ATTRIBUTE_HIDDEN
	}
}

// SetSystem sets or clears the system attribute.
func (wa *WindowsAttributes) SetSystem(system bool) {
	if system {
		wa.attrs |= FILE_ATTRIBUTE_SYSTEM
	} else {
		wa.attrs &^= FILE_ATTRIBUTE_SYSTEM
	}
}

// SetReadOnly sets or clears the read-only attribute.
func (wa *WindowsAttributes) SetReadOnly(readonly bool) {
	if readonly {
		wa.attrs |= FILE_ATTRIBUTE_READONLY
	} else {
		wa.attrs &^= FILE_ATTRIBUTE_READONLY
	}
}

// SetArchive sets or clears the archive attribute.
func (wa *WindowsAttributes) SetArchive(archive bool) {
	if archive {
		wa.attrs |= FILE_ATTRIBUTE_ARCHIVE
	} else {
		wa.attrs &^= FILE_ATTRIBUTE_ARCHIVE
	}
}

// String returns a human-readable string of the attributes.
func (wa *WindowsAttributes) String() string {
	var attrs []string

	if wa.IsReadOnly() {
		attrs = append(attrs, "ReadOnly")
	}
	if wa.IsHidden() {
		attrs = append(attrs, "Hidden")
	}
	if wa.IsSystem() {
		attrs = append(attrs, "System")
	}
	if wa.IsArchive() {
		attrs = append(attrs, "Archive")
	}
	if wa.IsTemporary() {
		attrs = append(attrs, "Temporary")
	}
	if wa.IsSparse() {
		attrs = append(attrs, "Sparse")
	}
	if wa.IsReparsePoint() {
		attrs = append(attrs, "ReparsePoint")
	}
	if wa.IsCompressed() {
		attrs = append(attrs, "Compressed")
	}
	if wa.IsOffline() {
		attrs = append(attrs, "Offline")
	}
	if wa.IsEncrypted() {
		attrs = append(attrs, "Encrypted")
	}

	if len(attrs) == 0 {
		return "Normal"
	}

	result := ""
	for i, attr := range attrs {
		if i > 0 {
			result += ", "
		}
		result += attr
	}
	return result
}

// FileInfoEx extends fs.FileInfo with Windows attributes.
type FileInfoEx interface {
	fs.FileInfo
	// WindowsAttributes returns the Windows-specific file attributes.
	// Returns nil if attributes are not available.
	WindowsAttributes() *WindowsAttributes
}

// GetWindowsAttributes attempts to extract Windows attributes from fs.FileInfo.
// Returns nil if the FileInfo doesn't support Windows attributes.
func GetWindowsAttributes(info fs.FileInfo) *WindowsAttributes {
	if infoEx, ok := info.(FileInfoEx); ok {
		return infoEx.WindowsAttributes()
	}

	// Try to extract from os.FileInfo.Sys()
	// On Windows, sys contains *syscall.Win32FileAttributeData
	// On Unix with SMB, we might get other types
	// This is a placeholder for potential future extraction
	_ = info.Sys()

	return nil
}

// attributesToMode converts Windows attributes to Unix file mode.
// This is a best-effort mapping as Windows and Unix permissions are quite different.
func attributesToMode(attrs uint32, isDir bool) fs.FileMode {
	mode := fs.FileMode(0666) // Default: read-write for all

	// If read-only, remove write permissions
	if attrs&FILE_ATTRIBUTE_READONLY != 0 {
		mode = fs.FileMode(0444) // Read-only for all
	}

	// Add directory flag
	if isDir || attrs&FILE_ATTRIBUTE_DIRECTORY != 0 {
		if attrs&FILE_ATTRIBUTE_READONLY != 0 {
			mode = fs.ModeDir | 0555 // Read/execute for all
		} else {
			mode = fs.ModeDir | 0777 // Full permissions
		}
	}

	// Add special flags
	if attrs&FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		mode |= fs.ModeSymlink
	}

	if attrs&FILE_ATTRIBUTE_DEVICE != 0 {
		mode |= fs.ModeDevice
	}

	return mode
}

// modeToAttributes converts Unix file mode to Windows attributes.
// This is a best-effort mapping as Windows and Unix permissions are quite different.
func modeToAttributes(mode fs.FileMode) uint32 {
	attrs := uint32(FILE_ATTRIBUTE_NORMAL)

	// Check if read-only (no write permissions)
	if mode&0222 == 0 {
		attrs |= FILE_ATTRIBUTE_READONLY
	}

	// Directory
	if mode.IsDir() {
		attrs |= FILE_ATTRIBUTE_DIRECTORY
	}

	// Symlink -> Reparse point
	if mode&fs.ModeSymlink != 0 {
		attrs |= FILE_ATTRIBUTE_REPARSE_POINT
	}

	// Device
	if mode&fs.ModeDevice != 0 {
		attrs |= FILE_ATTRIBUTE_DEVICE
	}

	// Archive (set by default for regular files)
	if mode.IsRegular() {
		attrs |= FILE_ATTRIBUTE_ARCHIVE
	}

	return attrs
}
