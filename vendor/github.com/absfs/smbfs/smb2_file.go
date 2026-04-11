package smbfs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/absfs/absfs"
)

// handleCreate processes an SMB2 CREATE request
// This is the most complex file operation, handling file/directory creation and opening
func (h *SMBHandler) handleCreate(state *connState, msg *SMB2Message, respHeader *SMB2Header) ([]byte, NTStatus) {
	// Validate session and tree
	session, tree, status := h.validateTree(msg.Header)
	if status != STATUS_SUCCESS {
		return h.buildErrorResponse(), status
	}

	// Parse request - minimum size is 57 bytes
	if len(msg.Payload) < 56 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	r := NewByteReader(msg.Payload)
	structSize := r.ReadUint16()
	if structSize != 57 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	securityFlags := r.ReadOneByte()
	oplockLevel := r.ReadOneByte()
	impersonationLevel := r.ReadUint32()
	createFlags := r.ReadUint64()
	_ = r.ReadUint64() // Reserved
	desiredAccess := r.ReadUint32()
	fileAttributes := r.ReadUint32()
	shareAccess := r.ReadUint32()
	createDisposition := r.ReadUint32()
	createOptions := r.ReadUint32()
	nameOffset := r.ReadUint16()
	nameLength := r.ReadUint16()
	_ = r.ReadUint32() // CreateContextsOffset
	_ = r.ReadUint32() // CreateContextsLength

	// Extract filename from UTF-16LE buffer
	// nameOffset is relative to the start of the SMB2 header
	nameStart := int(nameOffset) - SMB2HeaderSize
	if nameStart < 0 || nameStart+int(nameLength) > len(msg.Payload) {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}
	filename := DecodeUTF16LEToString(msg.Payload[nameStart : nameStart+int(nameLength)])

	// Convert backslashes to forward slashes
	filename = strings.ReplaceAll(filename, "\\", "/")
	// Remove leading slash if present
	filename = strings.TrimPrefix(filename, "/")
	// Empty path means root directory
	if filename == "" {
		filename = "/"
	}

	h.server.logger.Debug("CREATE: path=%s, disposition=0x%x, access=0x%x, share=0x%x, options=0x%x",
		filename, createDisposition, desiredAccess, shareAccess, createOptions)

	// Suppress unused variable warnings
	_ = securityFlags
	_ = oplockLevel
	_ = impersonationLevel
	_ = createFlags
	_ = fileAttributes

	// Check if this is a directory operation
	wantDir := createOptions&FILE_DIRECTORY_FILE != 0
	wantFile := createOptions&FILE_NON_DIRECTORY_FILE != 0
	deleteOnClose := createOptions&FILE_DELETE_ON_CLOSE != 0

	// Check share access compatibility with existing opens
	if !tree.Share.fileHandles.CheckShareAccess(filename, desiredAccess, shareAccess) {
		h.server.logger.Debug("CREATE: sharing violation for %s", filename)
		return h.buildErrorResponse(), STATUS_SHARING_VIOLATION
	}

	// Determine open mode based on create disposition
	var file absfs.File
	var err error
	var createAction uint32
	var existed bool

	// First, check if file exists
	info, statErr := tree.Share.fs.Stat(filename)
	existed = statErr == nil

	// Handle create dispositions
	switch createDisposition {
	case FILE_OPEN:
		// Open existing file; fail if not exists
		if !existed {
			return h.buildErrorResponse(), STATUS_OBJECT_NAME_NOT_FOUND
		}
		// Check if type matches expectations
		if wantDir && !info.IsDir() {
			return h.buildErrorResponse(), STATUS_NOT_A_DIRECTORY
		}
		if wantFile && info.IsDir() {
			return h.buildErrorResponse(), STATUS_FILE_IS_A_DIRECTORY
		}
		file, err = tree.Share.fs.OpenFile(filename, os.O_RDWR, 0)
		if err != nil {
			// Try read-only if write fails
			file, err = tree.Share.fs.OpenFile(filename, os.O_RDONLY, 0)
		}
		createAction = FILE_OPENED

	case FILE_CREATE:
		// Create new file; fail if exists
		if existed {
			return h.buildErrorResponse(), STATUS_OBJECT_NAME_COLLISION
		}
		if tree.IsReadOnly {
			return h.buildErrorResponse(), STATUS_ACCESS_DENIED
		}
		if wantDir {
			// Create directory
			err = tree.Share.fs.Mkdir(filename, 0755)
			if err == nil {
				file, err = tree.Share.fs.OpenFile(filename, os.O_RDONLY, 0)
			}
		} else {
			// Create file
			file, err = tree.Share.fs.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
		}
		createAction = FILE_CREATED

	case FILE_OPEN_IF:
		// Open if exists; create if not
		if existed {
			// Check if type matches expectations
			if wantDir && !info.IsDir() {
				return h.buildErrorResponse(), STATUS_NOT_A_DIRECTORY
			}
			if wantFile && info.IsDir() {
				return h.buildErrorResponse(), STATUS_FILE_IS_A_DIRECTORY
			}
			file, err = tree.Share.fs.OpenFile(filename, os.O_RDWR, 0)
			if err != nil {
				file, err = tree.Share.fs.OpenFile(filename, os.O_RDONLY, 0)
			}
			createAction = FILE_OPENED
		} else {
			if tree.IsReadOnly {
				return h.buildErrorResponse(), STATUS_ACCESS_DENIED
			}
			if wantDir {
				err = tree.Share.fs.Mkdir(filename, 0755)
				if err == nil {
					file, err = tree.Share.fs.OpenFile(filename, os.O_RDONLY, 0)
				}
			} else {
				file, err = tree.Share.fs.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
			}
			createAction = FILE_CREATED
		}

	case FILE_OVERWRITE:
		// Open and overwrite; fail if not exists
		if !existed {
			return h.buildErrorResponse(), STATUS_OBJECT_NAME_NOT_FOUND
		}
		if tree.IsReadOnly {
			return h.buildErrorResponse(), STATUS_ACCESS_DENIED
		}
		if info.IsDir() {
			return h.buildErrorResponse(), STATUS_FILE_IS_A_DIRECTORY
		}
		file, err = tree.Share.fs.OpenFile(filename, os.O_RDWR|os.O_TRUNC, 0644)
		createAction = FILE_OVERWRITTEN

	case FILE_OVERWRITE_IF:
		// Open and overwrite; create if not exists
		if tree.IsReadOnly {
			return h.buildErrorResponse(), STATUS_ACCESS_DENIED
		}
		if existed && info.IsDir() {
			return h.buildErrorResponse(), STATUS_FILE_IS_A_DIRECTORY
		}
		file, err = tree.Share.fs.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
		if existed {
			createAction = FILE_OVERWRITTEN
		} else {
			createAction = FILE_CREATED
		}

	case FILE_SUPERSEDE:
		// Replace if exists; create if not
		if tree.IsReadOnly {
			return h.buildErrorResponse(), STATUS_ACCESS_DENIED
		}
		if existed && info.IsDir() {
			return h.buildErrorResponse(), STATUS_FILE_IS_A_DIRECTORY
		}
		file, err = tree.Share.fs.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
		if existed {
			createAction = FILE_SUPERSEDED
		} else {
			createAction = FILE_CREATED
		}

	default:
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	// Check for errors
	if err != nil {
		h.server.logger.Debug("CREATE: failed to open %s: %v", filename, err)
		return h.buildErrorResponse(), mapGoErrorToNTStatus(err)
	}

	// Get file info
	info, err = file.Stat()
	if err != nil {
		file.Close()
		return h.buildErrorResponse(), mapGoErrorToNTStatus(err)
	}

	// Allocate file handle
	of := tree.Share.fileHandles.Allocate(
		file,
		filename,
		info.IsDir(),
		desiredAccess,
		shareAccess,
		createDisposition,
		createOptions,
		tree.ID,
		session.ID,
	)

	// Set delete on close flag if requested
	if deleteOnClose {
		of.DeleteOnClose = true
	}

	h.server.logger.Info("File opened: %s (FileID=%d/%d, Action=%d, Size=%d)",
		filename, of.ID.Persistent, of.ID.Volatile, createAction, info.Size())

	// Build response (structure size 89)
	w := NewByteWriter(256)
	w.WriteUint16(89) // StructureSize
	w.WriteOneByte(0)    // OplockLevel (none)
	w.WriteOneByte(0)    // Flags (reserved)
	w.WriteUint32(createAction)

	// File times
	w.WriteUint64(TimeToFiletime(info.ModTime())) // CreationTime
	w.WriteUint64(TimeToFiletime(info.ModTime())) // LastAccessTime
	w.WriteUint64(TimeToFiletime(info.ModTime())) // LastWriteTime
	w.WriteUint64(TimeToFiletime(info.ModTime())) // ChangeTime

	// File size and attributes
	size := info.Size()
	allocationSize := (size + 4095) &^ 4095 // Round up to 4KB
	w.WriteUint64(uint64(allocationSize))   // AllocationSize
	w.WriteUint64(uint64(size))             // EndOfFile

	// File attributes
	attrs := uint32(FILE_ATTRIBUTE_NORMAL)
	if info.IsDir() {
		attrs = FILE_ATTRIBUTE_DIRECTORY
	}
	if strings.HasPrefix(info.Name(), ".") {
		attrs |= FILE_ATTRIBUTE_HIDDEN
	}
	w.WriteUint32(attrs)

	w.WriteUint32(0) // Reserved2
	w.WriteFileID(of.ID)
	w.WriteUint32(0) // CreateContextsOffset
	w.WriteUint32(0) // CreateContextsLength

	return w.Bytes(), STATUS_SUCCESS
}

// handleClose processes an SMB2 CLOSE request
func (h *SMBHandler) handleClose(state *connState, msg *SMB2Message) ([]byte, NTStatus) {
	// Validate session and tree
	session, tree, status := h.validateTree(msg.Header)
	if status != STATUS_SUCCESS {
		return h.buildErrorResponse(), status
	}

	// Parse request - minimum size is 24 bytes
	if len(msg.Payload) < 24 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	r := NewByteReader(msg.Payload)
	structSize := r.ReadUint16()
	if structSize != 24 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	flags := r.ReadUint16()
	_ = r.ReadUint32() // Reserved
	fileID := r.ReadFileID()

	// Get file handle
	of := tree.Share.fileHandles.GetByTree(fileID, tree.ID, session.ID)
	if of == nil {
		return h.buildErrorResponse(), STATUS_FILE_CLOSED
	}

	h.server.logger.Debug("CLOSE: %s (FileID=%d/%d, flags=0x%x)",
		of.Path, fileID.Persistent, fileID.Volatile, flags)

	// Get file info before closing (if requested)
	var info fs.FileInfo
	var err error
	if flags&0x0001 != 0 { // SMB2_CLOSE_FLAG_POSTQUERY_ATTRIB
		info, err = of.File.Stat()
	}

	// Handle delete on close
	deleteOnClose := of.DeleteOnClose
	path := of.Path

	// Release the file handle (this closes the underlying file)
	if err := tree.Share.fileHandles.Release(fileID); err != nil {
		h.server.logger.Warn("CLOSE: failed to close file: %v", err)
	}

	// Delete file if requested
	if deleteOnClose {
		h.server.logger.Debug("CLOSE: deleting file on close: %s", path)
		if of.IsDir {
			err = tree.Share.fs.Remove(path)
		} else {
			err = tree.Share.fs.Remove(path)
		}
		if err != nil {
			h.server.logger.Warn("CLOSE: failed to delete file %s: %v", path, err)
		}
	}

	h.server.logger.Info("File closed: %s", path)

	// Build response (structure size 60)
	w := NewByteWriter(60)
	w.WriteUint16(60)    // StructureSize
	w.WriteUint16(flags) // Flags
	w.WriteUint32(0)     // Reserved

	// If info was requested and available, return it
	if info != nil && err == nil {
		w.WriteUint64(TimeToFiletime(info.ModTime())) // CreationTime
		w.WriteUint64(TimeToFiletime(info.ModTime())) // LastAccessTime
		w.WriteUint64(TimeToFiletime(info.ModTime())) // LastWriteTime
		w.WriteUint64(TimeToFiletime(info.ModTime())) // ChangeTime

		size := info.Size()
		allocationSize := (size + 4095) &^ 4095
		w.WriteUint64(uint64(allocationSize)) // AllocationSize
		w.WriteUint64(uint64(size))           // EndOfFile

		attrs := uint32(FILE_ATTRIBUTE_NORMAL)
		if info.IsDir() {
			attrs = FILE_ATTRIBUTE_DIRECTORY
		}
		w.WriteUint32(attrs)
	} else {
		// Return zeros if no info requested (times, sizes, attributes)
		// 4 times (4*8=32) + 2 sizes (2*8=16) + 1 attrs (4) = 52 bytes
		w.WriteZeros(52)
	}

	return w.Bytes(), STATUS_SUCCESS
}

// handleRead processes an SMB2 READ request
func (h *SMBHandler) handleRead(state *connState, msg *SMB2Message) ([]byte, NTStatus) {
	// Validate session and tree
	session, tree, status := h.validateTree(msg.Header)
	if status != STATUS_SUCCESS {
		return h.buildErrorResponse(), status
	}

	// Parse request - minimum size is 49 bytes
	if len(msg.Payload) < 48 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	r := NewByteReader(msg.Payload)
	structSize := r.ReadUint16()
	if structSize != 49 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	padding := r.ReadOneByte()
	flags := r.ReadOneByte()
	length := r.ReadUint32()
	offset := r.ReadUint64()
	fileID := r.ReadFileID()
	minCount := r.ReadUint32()
	_ = r.ReadUint32() // Channel
	_ = r.ReadUint32() // RemainingBytes
	_ = r.ReadUint16() // ReadChannelInfoOffset
	_ = r.ReadUint16() // ReadChannelInfoLength

	// Suppress unused variable warnings
	_ = padding
	_ = flags
	_ = minCount

	// Get file handle
	of := tree.Share.fileHandles.GetByTree(fileID, tree.ID, session.ID)
	if of == nil {
		return h.buildErrorResponse(), STATUS_FILE_CLOSED
	}

	// Update last access time
	tree.Share.fileHandles.UpdateLastAccess(fileID)

	// Limit read size to configured maximum
	if length > h.server.options.MaxReadSize {
		length = h.server.options.MaxReadSize
	}

	h.server.logger.Debug("READ: %s offset=%d length=%d", of.Path, offset, length)

	// Seek to offset
	if seeker, ok := of.File.(io.Seeker); ok {
		_, err := seeker.Seek(int64(offset), io.SeekStart)
		if err != nil {
			return h.buildErrorResponse(), mapGoErrorToNTStatus(err)
		}
	}

	// Read data
	buf := make([]byte, length)
	n, err := of.File.Read(buf)
	if err != nil && err != io.EOF {
		h.server.logger.Debug("READ: failed to read from %s: %v", of.Path, err)
		return h.buildErrorResponse(), mapGoErrorToNTStatus(err)
	}
	buf = buf[:n]

	h.server.logger.Debug("READ: read %d bytes from %s", n, of.Path)

	// If we read 0 bytes and got EOF, return end of file status
	if n == 0 && (err == io.EOF || err == nil) {
		return h.buildErrorResponse(), STATUS_END_OF_FILE
	}

	// Build response (structure size 17)
	// Data starts at: SMB2 header (64) + response fields (16) = offset 80
	// The DataOffset field is 1 byte telling client where data starts from header
	dataOffset := uint8(SMB2HeaderSize + 16)

	w := NewByteWriter(17 + n)
	w.WriteUint16(17)          // StructureSize (bytes 0-1)
	w.WriteOneByte(dataOffset) // DataOffset (byte 2)
	w.WriteOneByte(0)          // Reserved (byte 3)
	w.WriteUint32(uint32(n))   // DataLength (bytes 4-7)
	w.WriteUint32(0)           // DataRemaining (bytes 8-11)
	w.WriteUint32(0)           // Reserved2 (bytes 12-15)
	w.WriteBytes(buf)          // Data (bytes 16+)

	return w.Bytes(), STATUS_SUCCESS
}

// handleWrite processes an SMB2 WRITE request
func (h *SMBHandler) handleWrite(state *connState, msg *SMB2Message) ([]byte, NTStatus) {
	// Validate session and tree
	session, tree, status := h.validateTree(msg.Header)
	if status != STATUS_SUCCESS {
		return h.buildErrorResponse(), status
	}

	// Parse request - minimum size is 49 bytes
	if len(msg.Payload) < 48 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	r := NewByteReader(msg.Payload)
	structSize := r.ReadUint16()
	if structSize != 49 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	dataOffset := r.ReadUint16()
	length := r.ReadUint32()
	offset := r.ReadUint64()
	fileID := r.ReadFileID()
	_ = r.ReadUint32() // Channel
	_ = r.ReadUint32() // RemainingBytes
	_ = r.ReadUint16() // WriteChannelInfoOffset
	_ = r.ReadUint16() // WriteChannelInfoLength
	flags := r.ReadUint32()

	// Suppress unused variable warnings
	_ = flags

	// Get file handle
	of := tree.Share.fileHandles.GetByTree(fileID, tree.ID, session.ID)
	if of == nil {
		return h.buildErrorResponse(), STATUS_FILE_CLOSED
	}

	// Check if tree/file is read-only
	if tree.IsReadOnly {
		return h.buildErrorResponse(), STATUS_ACCESS_DENIED
	}

	// Check if handle has write access
	// Map generic access to specific access
	access := mapGenericAccess(of.Access)
	if access&(FILE_WRITE_DATA|FILE_APPEND_DATA) == 0 {
		return h.buildErrorResponse(), STATUS_ACCESS_DENIED
	}

	// Update last access time
	tree.Share.fileHandles.UpdateLastAccess(fileID)

	// Extract write data
	// dataOffset is relative to the start of the SMB2 header
	dataStart := int(dataOffset) - SMB2HeaderSize
	if dataStart < 0 || dataStart+int(length) > len(msg.Payload) {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}
	data := msg.Payload[dataStart : dataStart+int(length)]

	h.server.logger.Debug("WRITE: %s offset=%d length=%d", of.Path, offset, length)

	// Seek to offset
	if seeker, ok := of.File.(io.Seeker); ok {
		_, err := seeker.Seek(int64(offset), io.SeekStart)
		if err != nil {
			return h.buildErrorResponse(), mapGoErrorToNTStatus(err)
		}
	}

	// Write data
	n, err := of.File.Write(data)
	if err != nil {
		h.server.logger.Debug("WRITE: failed to write to %s: %v", of.Path, err)
		return h.buildErrorResponse(), mapGoErrorToNTStatus(err)
	}

	h.server.logger.Debug("WRITE: wrote %d bytes to %s", n, of.Path)

	// Build response (structure size 17)
	w := NewByteWriter(17)
	w.WriteUint16(17)          // StructureSize
	w.WriteUint16(0)           // Reserved
	w.WriteUint32(uint32(n))   // Count
	w.WriteUint32(0)           // Remaining
	w.WriteUint16(0)           // WriteChannelInfoOffset
	w.WriteUint16(0)           // WriteChannelInfoLength

	return w.Bytes(), STATUS_SUCCESS
}

// handleFlush processes an SMB2 FLUSH request
func (h *SMBHandler) handleFlush(state *connState, msg *SMB2Message) ([]byte, NTStatus) {
	// Validate session and tree
	session, tree, status := h.validateTree(msg.Header)
	if status != STATUS_SUCCESS {
		return h.buildErrorResponse(), status
	}

	// Parse request - minimum size is 24 bytes
	if len(msg.Payload) < 24 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	r := NewByteReader(msg.Payload)
	structSize := r.ReadUint16()
	if structSize != 24 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	_ = r.ReadUint16() // Reserved1
	_ = r.ReadUint32() // Reserved2
	fileID := r.ReadFileID()

	// Get file handle
	of := tree.Share.fileHandles.GetByTree(fileID, tree.ID, session.ID)
	if of == nil {
		return h.buildErrorResponse(), STATUS_FILE_CLOSED
	}

	h.server.logger.Debug("FLUSH: %s", of.Path)

	// Sync file if it implements Sync()
	type syncer interface {
		Sync() error
	}

	if s, ok := of.File.(syncer); ok {
		if err := s.Sync(); err != nil {
			h.server.logger.Debug("FLUSH: failed to sync %s: %v", of.Path, err)
			return h.buildErrorResponse(), mapGoErrorToNTStatus(err)
		}
	}

	h.server.logger.Debug("FLUSH: synced %s", of.Path)

	// Build response (structure size 4)
	w := NewByteWriter(4)
	w.WriteUint16(4) // StructureSize
	w.WriteUint16(0) // Reserved

	return w.Bytes(), STATUS_SUCCESS
}

// mapGenericAccess maps generic access rights to specific file access rights
func mapGenericAccess(access uint32) uint32 {
	result := access

	// Map GENERIC_READ to specific read rights
	if access&GENERIC_READ != 0 {
		result |= FILE_READ_DATA | FILE_READ_ATTRIBUTES | FILE_READ_EA | READ_CONTROL | SYNCHRONIZE
	}

	// Map GENERIC_WRITE to specific write rights
	if access&GENERIC_WRITE != 0 {
		result |= FILE_WRITE_DATA | FILE_APPEND_DATA | FILE_WRITE_ATTRIBUTES | FILE_WRITE_EA | SYNCHRONIZE
	}

	// Map GENERIC_EXECUTE to specific execute rights
	if access&GENERIC_EXECUTE != 0 {
		result |= FILE_EXECUTE | FILE_READ_ATTRIBUTES | SYNCHRONIZE
	}

	// Map GENERIC_ALL to all rights
	if access&GENERIC_ALL != 0 {
		result |= FILE_READ_DATA | FILE_WRITE_DATA | FILE_APPEND_DATA |
			FILE_READ_EA | FILE_WRITE_EA | FILE_EXECUTE | FILE_DELETE_CHILD |
			FILE_READ_ATTRIBUTES | FILE_WRITE_ATTRIBUTES | DELETE | READ_CONTROL |
			WRITE_DAC | WRITE_OWNER | SYNCHRONIZE
	}

	return result
}

// mapGoErrorToNTStatus maps Go errors to NT status codes
func mapGoErrorToNTStatus(err error) NTStatus {
	if err == nil {
		return STATUS_SUCCESS
	}

	// Check for standard fs errors
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return STATUS_OBJECT_NAME_NOT_FOUND
	case errors.Is(err, fs.ErrExist):
		return STATUS_OBJECT_NAME_COLLISION
	case errors.Is(err, fs.ErrPermission):
		return STATUS_ACCESS_DENIED
	case errors.Is(err, fs.ErrInvalid):
		return STATUS_INVALID_PARAMETER
	case errors.Is(err, fs.ErrClosed):
		return STATUS_FILE_CLOSED
	case errors.Is(err, io.EOF):
		return STATUS_END_OF_FILE
	}

	// Check for os-specific errors
	switch {
	case errors.Is(err, ErrIsDirectory):
		return STATUS_FILE_IS_A_DIRECTORY
	case errors.Is(err, ErrNotDirectory):
		return STATUS_NOT_A_DIRECTORY
	}

	// Default to generic error
	return STATUS_INVALID_DEVICE_REQUEST
}
