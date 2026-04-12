package smbfs

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SMB2 QUERY_DIRECTORY flags
const (
	SMB2_RESTART_SCANS       uint8 = 0x01 // Restart directory enumeration
	SMB2_RETURN_SINGLE_ENTRY uint8 = 0x02 // Return only one entry
	SMB2_INDEX_SPECIFIED     uint8 = 0x04 // Start at FileIndex
	SMB2_REOPEN              uint8 = 0x10 // Reopen directory handle
)

// Directory enumeration state stored per file handle
type dirEnumState struct {
	entries   []os.FileInfo // Cached directory entries
	position  int           // Current position in entries
	pattern   string        // Search pattern
	exhausted bool          // True when no more entries
}

// handleQueryDirectory implements SMB2 QUERY_DIRECTORY command
func (h *SMBHandler) handleQueryDirectory(state *connState, msg *SMB2Message) ([]byte, NTStatus) {
	// Validate session and tree
	session, tree, status := h.validateTree(msg.Header)
	if status != STATUS_SUCCESS {
		return h.buildErrorResponse(), status
	}

	// Parse request
	if len(msg.Payload) < 32 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	r := NewByteReader(msg.Payload)
	structSize := r.ReadUint16()
	if structSize != 33 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	infoClass := r.ReadOneByte()
	flags := r.ReadOneByte()
	fileIndex := r.ReadUint32()
	fileID := r.ReadFileID()
	fileNameOffset := r.ReadUint16()
	fileNameLength := r.ReadUint16()
	outputBufferLength := r.ReadUint32()

	// Get file handle
	of := tree.Share.fileHandles.GetByTree(fileID, tree.ID, session.ID)
	if of == nil {
		return h.buildErrorResponse(), STATUS_FILE_CLOSED
	}

	// Verify this is a directory
	if !of.IsDir {
		return h.buildErrorResponse(), STATUS_NOT_A_DIRECTORY
	}

	// Parse search pattern
	var pattern string
	if fileNameLength > 0 {
		patternStart := int(fileNameOffset) - SMB2HeaderSize
		if patternStart >= 0 && patternStart+int(fileNameLength) <= len(msg.Payload) {
			pattern = DecodeUTF16LEToString(msg.Payload[patternStart : patternStart+int(fileNameLength)])
		}
	}
	if pattern == "" {
		pattern = "*" // Default to all files
	}

	h.server.logger.Debug("QUERY_DIRECTORY: path=%s, pattern=%s, class=%d, flags=0x%02x",
		of.Path, pattern, infoClass, flags)

	// Get or create directory enumeration state
	dirState := h.getDirState(of)
	if dirState == nil {
		dirState = &dirEnumState{pattern: pattern}
	}

	// Handle restart flag
	if flags&SMB2_RESTART_SCANS != 0 {
		dirState.position = 0
		dirState.exhausted = false
		dirState.pattern = pattern
		dirState.entries = nil
	}

	// Handle pattern change
	if dirState.pattern != pattern {
		dirState.pattern = pattern
		dirState.position = 0
		dirState.exhausted = false
		dirState.entries = nil
	}

	// Handle index specified
	if flags&SMB2_INDEX_SPECIFIED != 0 {
		dirState.position = int(fileIndex)
		dirState.exhausted = false
	}

	// If directory is exhausted, return NO_MORE_FILES
	if dirState.exhausted {
		h.storeDirState(of, dirState)
		return h.buildErrorResponse(), STATUS_NO_MORE_FILES
	}

	// Read directory entries if not cached
	if dirState.entries == nil {
		entries, err := h.readDirEntries(of, tree)
		if err != nil {
			h.server.logger.Error("Failed to read directory %s: %v", of.Path, err)
			return h.buildErrorResponse(), STATUS_ACCESS_DENIED
		}
		dirState.entries = entries
		dirState.position = 0
	}

	// Filter entries by pattern
	matchedEntries := h.filterEntries(dirState.entries[dirState.position:], dirState.pattern)
	if len(matchedEntries) == 0 {
		dirState.exhausted = true
		h.storeDirState(of, dirState)
		return h.buildErrorResponse(), STATUS_NO_MORE_FILES
	}

	// Build response buffer
	w := NewByteWriter(int(outputBufferLength))
	entryCount := 0
	entryOffsets := make([]int, 0, 16) // Track start offsets of each entry
	singleEntry := flags&SMB2_RETURN_SINGLE_ENTRY != 0

	for _, entry := range matchedEntries {
		// Format entry based on information class
		entryData := h.formatDirEntry(entry, infoClass, uint32(dirState.position+entryCount))
		if entryData == nil {
			// Unsupported info class
			h.storeDirState(of, dirState)
			return h.buildErrorResponse(), STATUS_NOT_SUPPORTED
		}

		// Calculate aligned size for this entry
		alignedSize := AlignTo8(len(entryData))

		// Check if entry fits in output buffer
		if w.Len()+alignedSize > int(outputBufferLength) {
			// Buffer would overflow
			if entryCount == 0 {
				// Can't fit even one entry
				h.storeDirState(of, dirState)
				return h.buildErrorResponse(), STATUS_BUFFER_OVERFLOW
			}
			// Return what we have so far
			break
		}

		// Track this entry's offset
		entryStart := w.Len()
		entryOffsets = append(entryOffsets, entryStart)

		// Backpatch NextEntryOffset in previous entry
		if entryCount > 0 {
			prevStart := entryOffsets[entryCount-1]
			offsetToCurrent := entryStart - prevStart
			w.SetUint32At(prevStart, uint32(offsetToCurrent))
		}

		w.WriteBytes(entryData)
		entryCount++
		dirState.position++

		// Align to 8-byte boundary for next entry
		w.WritePadTo8()

		if singleEntry {
			break
		}
	}

	// Set NextEntryOffset to 0 for last entry (already 0 from formatDirEntry)
	// No need to patch - formatDirEntry sets it to 0

	// Check if directory is exhausted
	if dirState.position >= len(dirState.entries) {
		dirState.exhausted = true
	}

	// Store updated state
	h.storeDirState(of, dirState)

	// Build response
	resp := NewByteWriter(9 + w.Len())
	resp.WriteUint16(9)                  // StructureSize
	resp.WriteUint16(SMB2HeaderSize + 8) // OutputBufferOffset
	resp.WriteUint32(uint32(w.Len()))    // OutputBufferLength
	resp.WriteBytes(w.Bytes())           // Buffer

	h.server.logger.Debug("QUERY_DIRECTORY: returned %d entries", entryCount)
	return resp.Bytes(), STATUS_SUCCESS
}

// readDirEntries reads all entries from a directory
func (h *SMBHandler) readDirEntries(of *OpenFile, tree *TreeConnection) ([]os.FileInfo, error) {
	// Read all directory entries
	dirEntries, err := of.File.ReadDir(-1)
	if err != nil && err != io.EOF {
		return nil, err
	}

	// Convert DirEntry to FileInfo
	var infos []os.FileInfo
	for _, entry := range dirEntries {
		info, err := entry.Info()
		if err != nil {
			// Skip entries we can't stat
			continue
		}
		infos = append(infos, info)
	}

	return infos, nil
}

// filterEntries filters directory entries by pattern
func (h *SMBHandler) filterEntries(entries []os.FileInfo, pattern string) []os.FileInfo {
	// Special case: "*" matches everything
	if pattern == "*" {
		return entries
	}

	var matched []os.FileInfo
	for _, entry := range entries {
		if matchPattern(entry.Name(), pattern) {
			matched = append(matched, entry)
		}
	}
	return matched
}

// formatDirEntry formats a directory entry according to the information class
func (h *SMBHandler) formatDirEntry(info os.FileInfo, infoClass uint8, fileIndex uint32) []byte {
	name := info.Name()
	nameUTF16 := EncodeStringToUTF16LE(name)
	nameLen := len(nameUTF16)

	// Get file attributes
	attrs := modeToAttributes(info.Mode())

	// Get timestamps
	modTime := info.ModTime()
	createTime := TimeToFiletime(modTime)
	lastAccess := TimeToFiletime(modTime)
	lastWrite := TimeToFiletime(modTime)
	changeTime := TimeToFiletime(modTime)

	// Get file size
	fileSize := uint64(info.Size())
	allocSize := (fileSize + 4095) &^ 4095 // Round up to 4KB boundary

	// If directory, size is 0
	if info.IsDir() {
		fileSize = 0
		allocSize = 0
	}

	switch infoClass {
	case FileDirectoryInformation:
		// FileDirectoryInformation: base structure
		w := NewByteWriter(64 + nameLen)
		w.WriteUint32(0)               // NextEntryOffset (backpatched later)
		w.WriteUint32(fileIndex)       // FileIndex
		w.WriteUint64(createTime)      // CreationTime
		w.WriteUint64(lastAccess)      // LastAccessTime
		w.WriteUint64(lastWrite)       // LastWriteTime
		w.WriteUint64(changeTime)      // ChangeTime
		w.WriteUint64(fileSize)        // EndOfFile
		w.WriteUint64(allocSize)       // AllocationSize
		w.WriteUint32(attrs)           // FileAttributes
		w.WriteUint32(uint32(nameLen)) // FileNameLength
		w.WriteBytes(nameUTF16)        // FileName
		return w.Bytes()

	case FileFullDirectoryInformation:
		// FileFullDirectoryInformation: adds EaSize
		w := NewByteWriter(68 + nameLen)
		w.WriteUint32(0)               // NextEntryOffset (backpatched later)
		w.WriteUint32(fileIndex)       // FileIndex
		w.WriteUint64(createTime)      // CreationTime
		w.WriteUint64(lastAccess)      // LastAccessTime
		w.WriteUint64(lastWrite)       // LastWriteTime
		w.WriteUint64(changeTime)      // ChangeTime
		w.WriteUint64(fileSize)        // EndOfFile
		w.WriteUint64(allocSize)       // AllocationSize
		w.WriteUint32(attrs)           // FileAttributes
		w.WriteUint32(uint32(nameLen)) // FileNameLength
		w.WriteUint32(0)               // EaSize (Extended Attributes)
		w.WriteBytes(nameUTF16)        // FileName
		return w.Bytes()

	case FileBothDirectoryInformation:
		// FileBothDirectoryInformation: adds ShortName
		w := NewByteWriter(94 + nameLen)
		w.WriteUint32(0)               // NextEntryOffset (backpatched later)
		w.WriteUint32(fileIndex)       // FileIndex
		w.WriteUint64(createTime)      // CreationTime
		w.WriteUint64(lastAccess)      // LastAccessTime
		w.WriteUint64(lastWrite)       // LastWriteTime
		w.WriteUint64(changeTime)      // ChangeTime
		w.WriteUint64(fileSize)        // EndOfFile
		w.WriteUint64(allocSize)       // AllocationSize
		w.WriteUint32(attrs)           // FileAttributes
		w.WriteUint32(uint32(nameLen)) // FileNameLength
		w.WriteUint32(0)               // EaSize
		w.WriteOneByte(0)                 // ShortNameLength (8.3 name)
		w.WriteOneByte(0)                 // Reserved
		w.WriteZeros(24)               // ShortName (12 UTF-16 chars)
		w.WriteBytes(nameUTF16)        // FileName
		return w.Bytes()

	case FileNamesInformation:
		// FileNamesInformation: names only (lightweight)
		w := NewByteWriter(12 + nameLen)
		w.WriteUint32(0)               // NextEntryOffset (backpatched later)
		w.WriteUint32(fileIndex)       // FileIndex
		w.WriteUint32(uint32(nameLen)) // FileNameLength
		w.WriteBytes(nameUTF16)        // FileName
		return w.Bytes()

	case FileIdBothDirectoryInformation:
		// FileIdBothDirectoryInformation: adds FileId (SMB 3.0+)
		w := NewByteWriter(104 + nameLen)
		w.WriteUint32(0)                 // NextEntryOffset (backpatched later)
		w.WriteUint32(fileIndex)         // FileIndex
		w.WriteUint64(createTime)        // CreationTime
		w.WriteUint64(lastAccess)        // LastAccessTime
		w.WriteUint64(lastWrite)         // LastWriteTime
		w.WriteUint64(changeTime)        // ChangeTime
		w.WriteUint64(fileSize)          // EndOfFile
		w.WriteUint64(allocSize)         // AllocationSize
		w.WriteUint32(attrs)             // FileAttributes
		w.WriteUint32(uint32(nameLen))   // FileNameLength
		w.WriteUint32(0)                 // EaSize
		w.WriteOneByte(0)                   // ShortNameLength
		w.WriteOneByte(0)                   // Reserved1
		w.WriteZeros(24)                 // ShortName (12 UTF-16 chars)
		w.WriteUint16(0)                 // Reserved2
		w.WriteUint64(uint64(fileIndex)) // FileId
		w.WriteBytes(nameUTF16)          // FileName
		return w.Bytes()

	default:
		// Unsupported information class
		return nil
	}
}

// matchPattern performs simple glob matching (*, ? wildcards)
func matchPattern(name, pattern string) bool {
	// Case-insensitive matching (Windows convention)
	name = strings.ToUpper(name)
	pattern = strings.ToUpper(pattern)

	// Use filepath.Match for glob matching
	matched, err := filepath.Match(pattern, name)
	if err != nil {
		return false
	}
	return matched
}

// Directory state management (stored per file handle)
// In a real implementation, this would be a map[FileID]*dirEnumState
// For now, we use a simple in-memory map

var (
	dirStates   = make(map[FileID]*dirEnumState)
	dirStatesMu sync.Mutex
)

func (h *SMBHandler) getDirState(of *OpenFile) *dirEnumState {
	dirStatesMu.Lock()
	defer dirStatesMu.Unlock()
	return dirStates[of.ID]
}

func (h *SMBHandler) storeDirState(of *OpenFile, state *dirEnumState) {
	dirStatesMu.Lock()
	defer dirStatesMu.Unlock()
	dirStates[of.ID] = state
}
