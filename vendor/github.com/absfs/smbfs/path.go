package smbfs

import (
	"path"
	"strings"
)

// pathNormalizer handles path normalization for SMB shares.
type pathNormalizer struct {
	caseSensitive bool
}

// newPathNormalizer creates a new path normalizer.
func newPathNormalizer(caseSensitive bool) *pathNormalizer {
	return &pathNormalizer{
		caseSensitive: caseSensitive,
	}
}

// normalize normalizes a path for use with SMB.
// Supported formats:
//   - Windows: \\server\share\path\to\file
//   - Unix-style: /path/to/file
//   - SMB URL: smb://server/share/path/to/file
func (pn *pathNormalizer) normalize(p string) string {
	// Empty path becomes root
	if p == "" {
		return "/"
	}

	// Convert Windows separators to forward slashes
	p = strings.ReplaceAll(p, "\\", "/")

	// Clean the path (removes .., ., multiple slashes, etc.)
	p = path.Clean(p)

	// path.Clean converts "." to ".", so handle it
	if p == "." {
		p = "/"
	}

	// Ensure the path starts with /
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	// Case normalization (Windows/SMB is typically case-insensitive)
	if !pn.caseSensitive {
		p = strings.ToLower(p)
	}

	return p
}

// join joins path components and normalizes the result.
func (pn *pathNormalizer) join(elem ...string) string {
	joined := path.Join(elem...)
	return pn.normalize(joined)
}

// dir returns the directory portion of the path.
func (pn *pathNormalizer) dir(p string) string {
	p = pn.normalize(p)
	return path.Dir(p)
}

// base returns the last element of the path.
func (pn *pathNormalizer) base(p string) string {
	p = pn.normalize(p)
	return path.Base(p)
}

// split splits the path into directory and file components.
func (pn *pathNormalizer) split(p string) (dir, file string) {
	p = pn.normalize(p)
	return path.Split(p)
}

// isAbs returns true if the path is absolute.
func isAbs(p string) bool {
	return strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\")
}

// validatePath validates that a path is safe and doesn't contain
// invalid characters or attempt path traversal outside the share.
func validatePath(p string) error {
	if p == "" {
		return ErrInvalidPath
	}

	// Check for null bytes
	if strings.Contains(p, "\x00") {
		return ErrInvalidPath
	}

	// After normalization, the path should be clean
	normalized := strings.ReplaceAll(p, "\\", "/")
	cleaned := path.Clean(normalized)

	// Path traversal check - ensure the clean path doesn't go above root
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "/..") {
		return ErrInvalidPath
	}

	return nil
}

// toSMBPath converts a normalized Unix-style path to SMB path format.
// SMB paths use backslashes and don't have a leading slash.
func toSMBPath(p string) string {
	// Remove leading slash
	p = strings.TrimPrefix(p, "/")

	// Convert to backslashes for SMB
	p = strings.ReplaceAll(p, "/", "\\")

	return p
}

// fromSMBPath converts an SMB path to normalized Unix-style path.
func fromSMBPath(p string) string {
	// Convert backslashes to forward slashes
	p = strings.ReplaceAll(p, "\\", "/")

	// Ensure leading slash
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	return path.Clean(p)
}
