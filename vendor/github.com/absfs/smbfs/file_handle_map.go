package smbfs

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/absfs/absfs"
)

// OpenFile represents an open file handle in the SMB server
type OpenFile struct {
	ID           FileID           // SMB2 file identifier
	File         absfs.File       // The underlying absfs file
	Path         string           // Path to the file (for debugging/logging)
	IsDir        bool             // True if this is a directory
	Access       uint32           // Access mask (read, write, etc.)
	ShareAccess  uint32           // Share access flags
	Disposition  uint32           // Create disposition used
	Options      uint32           // Create options used
	CreatedAt    time.Time        // When the handle was created
	LastAccess   time.Time        // Last access time
	TreeID       uint32           // Tree ID this handle belongs to
	SessionID    uint64           // Session ID this handle belongs to
	DeleteOnClose bool            // Delete file when handle is closed
}

// FileHandleMap manages SMB FileID to OpenFile mappings
// Thread-safe for concurrent access from multiple connections
type FileHandleMap struct {
	mu         sync.RWMutex
	handles    map[FileID]*OpenFile
	byPath     map[string][]*OpenFile // Track handles by path for sharing checks
	nextHandle uint64
}

// NewFileHandleMap creates a new file handle map
func NewFileHandleMap() *FileHandleMap {
	return &FileHandleMap{
		handles:    make(map[FileID]*OpenFile),
		byPath:     make(map[string][]*OpenFile),
		nextHandle: 1,
	}
}

// Allocate creates a new file handle for the given file
func (m *FileHandleMap) Allocate(file absfs.File, path string, isDir bool, access, shareAccess, disposition, options uint32, treeID uint32, sessionID uint64) *OpenFile {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate a random persistent ID and use sequential volatile ID
	var persistentID uint64
	var randomBytes [8]byte
	if _, err := rand.Read(randomBytes[:]); err == nil {
		persistentID = le.Uint64(randomBytes[:])
	}

	volatileID := m.nextHandle
	m.nextHandle++

	id := FileID{
		Persistent: persistentID,
		Volatile:   volatileID,
	}

	now := time.Now()
	of := &OpenFile{
		ID:          id,
		File:        file,
		Path:        path,
		IsDir:       isDir,
		Access:      access,
		ShareAccess: shareAccess,
		Disposition: disposition,
		Options:     options,
		CreatedAt:   now,
		LastAccess:  now,
		TreeID:      treeID,
		SessionID:   sessionID,
	}

	m.handles[id] = of
	m.byPath[path] = append(m.byPath[path], of)

	return of
}

// Get retrieves an open file by its FileID
func (m *FileHandleMap) Get(id FileID) *OpenFile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.handles[id]
}

// GetBySession retrieves an open file, validating it belongs to the given session
func (m *FileHandleMap) GetBySession(id FileID, sessionID uint64) *OpenFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	of := m.handles[id]
	if of == nil || of.SessionID != sessionID {
		return nil
	}
	return of
}

// GetByTree retrieves an open file, validating it belongs to the given tree
func (m *FileHandleMap) GetByTree(id FileID, treeID uint32, sessionID uint64) *OpenFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	of := m.handles[id]
	if of == nil || of.TreeID != treeID || of.SessionID != sessionID {
		return nil
	}
	return of
}

// Release closes and removes a file handle
func (m *FileHandleMap) Release(id FileID) error {
	m.mu.Lock()
	of := m.handles[id]
	if of == nil {
		m.mu.Unlock()
		return nil
	}

	delete(m.handles, id)

	// Remove from byPath tracking
	handles := m.byPath[of.Path]
	for i, h := range handles {
		if h.ID == id {
			m.byPath[of.Path] = append(handles[:i], handles[i+1:]...)
			break
		}
	}
	if len(m.byPath[of.Path]) == 0 {
		delete(m.byPath, of.Path)
	}

	m.mu.Unlock()

	// Close the underlying file
	if of.File != nil {
		return of.File.Close()
	}
	return nil
}

// ReleaseBySession releases all handles belonging to a session
func (m *FileHandleMap) ReleaseBySession(sessionID uint64) []error {
	m.mu.Lock()
	var toRelease []FileID
	for id, of := range m.handles {
		if of.SessionID == sessionID {
			toRelease = append(toRelease, id)
		}
	}
	m.mu.Unlock()

	var errors []error
	for _, id := range toRelease {
		if err := m.Release(id); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

// ReleaseByTree releases all handles belonging to a tree connection
func (m *FileHandleMap) ReleaseByTree(treeID uint32, sessionID uint64) []error {
	m.mu.Lock()
	var toRelease []FileID
	for id, of := range m.handles {
		if of.TreeID == treeID && of.SessionID == sessionID {
			toRelease = append(toRelease, id)
		}
	}
	m.mu.Unlock()

	var errors []error
	for _, id := range toRelease {
		if err := m.Release(id); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

// Count returns the number of open handles
func (m *FileHandleMap) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.handles)
}

// GetOpenHandlesForPath returns all open handles for a given path
func (m *FileHandleMap) GetOpenHandlesForPath(path string) []*OpenFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	handles := m.byPath[path]
	result := make([]*OpenFile, len(handles))
	copy(result, handles)
	return result
}

// CheckShareAccess verifies if a new open with the given access and share mode
// is compatible with existing opens on the same path
func (m *FileHandleMap) CheckShareAccess(path string, desiredAccess, shareAccess uint32) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	existing := m.byPath[path]
	for _, of := range existing {
		// Check if the new open's share mode allows existing access
		if !checkShareCompatibility(desiredAccess, shareAccess, of.Access, of.ShareAccess) {
			return false
		}
	}
	return true
}

// checkShareCompatibility checks if two opens are compatible
func checkShareCompatibility(newAccess, newShare, existingAccess, existingShare uint32) bool {
	// Check if new access is allowed by existing share mode
	if newAccess&FILE_READ_DATA != 0 && existingShare&FILE_SHARE_READ == 0 {
		return false
	}
	if newAccess&FILE_WRITE_DATA != 0 && existingShare&FILE_SHARE_WRITE == 0 {
		return false
	}
	if newAccess&DELETE != 0 && existingShare&FILE_SHARE_DELETE == 0 {
		return false
	}

	// Check if existing access is allowed by new share mode
	if existingAccess&FILE_READ_DATA != 0 && newShare&FILE_SHARE_READ == 0 {
		return false
	}
	if existingAccess&FILE_WRITE_DATA != 0 && newShare&FILE_SHARE_WRITE == 0 {
		return false
	}
	if existingAccess&DELETE != 0 && newShare&FILE_SHARE_DELETE == 0 {
		return false
	}

	return true
}

// UpdateLastAccess updates the last access time for a handle
func (m *FileHandleMap) UpdateLastAccess(id FileID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if of := m.handles[id]; of != nil {
		of.LastAccess = time.Now()
	}
}

// SetDeleteOnClose marks a file to be deleted when closed
func (m *FileHandleMap) SetDeleteOnClose(id FileID, deleteOnClose bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if of := m.handles[id]; of != nil {
		of.DeleteOnClose = deleteOnClose
	}
}

// GetDeleteOnClose returns whether a file should be deleted when closed
func (m *FileHandleMap) GetDeleteOnClose(id FileID) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if of := m.handles[id]; of != nil {
		return of.DeleteOnClose
	}
	return false
}
