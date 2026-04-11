package smbfs

import (
	"io/fs"
	"os"
	"path"
	"time"

	"github.com/absfs/absfs"
)

// handleQueryInfo handles SMB2 QUERY_INFO requests
func (h *SMBHandler) handleQueryInfo(state *connState, msg *SMB2Message) ([]byte, NTStatus) {
	// Validate session and tree
	session, tree, status := h.validateTree(msg.Header)
	if status != STATUS_SUCCESS {
		return h.buildErrorResponse(), status
	}

	// Parse request
	if len(msg.Payload) < 40 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	r := NewByteReader(msg.Payload)
	structSize := r.ReadUint16()
	if structSize != 41 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	infoType := r.ReadOneByte()
	fileInfoClass := r.ReadOneByte()
	outputBufferLength := r.ReadUint32()
	inputBufferOffset := r.ReadUint16()
	_ = r.ReadUint16() // Reserved
	inputBufferLength := r.ReadUint32()
	additionalInfo := r.ReadUint32()
	flags := r.ReadUint32()
	fileID := r.ReadFileID()

	// Suppress unused variable warnings
	_ = inputBufferOffset
	_ = inputBufferLength
	_ = additionalInfo
	_ = flags

	h.server.logger.Debug("QUERY_INFO: type=%d class=%d outputLen=%d fileID=%v",
		infoType, fileInfoClass, outputBufferLength, fileID)

	// Get the file handle
	of := tree.Share.fileHandles.GetByTree(fileID, tree.ID, session.ID)
	if of == nil {
		return h.buildErrorResponse(), STATUS_FILE_CLOSED
	}

	// Update last access time
	tree.Share.fileHandles.UpdateLastAccess(fileID)

	var buffer []byte

	switch infoType {
	case SMB2_0_INFO_FILE:
		buffer, status = h.queryFileInfo(of, fileInfoClass)
	case SMB2_0_INFO_FILESYSTEM:
		buffer, status = h.queryFilesystemInfo(tree.Share.fs, fileInfoClass)
	case SMB2_0_INFO_SECURITY:
		// Security info not supported yet
		return h.buildErrorResponse(), STATUS_NOT_SUPPORTED
	case SMB2_0_INFO_QUOTA:
		// Quota info not supported
		return h.buildErrorResponse(), STATUS_NOT_SUPPORTED
	default:
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	if status != STATUS_SUCCESS {
		return h.buildErrorResponse(), status
	}

	// Truncate buffer if needed
	if uint32(len(buffer)) > outputBufferLength {
		buffer = buffer[:outputBufferLength]
		status = STATUS_BUFFER_OVERFLOW
	}

	// Build response
	w := NewByteWriter(64 + len(buffer))
	w.WriteUint16(9)                          // StructureSize
	w.WriteUint16(uint16(SMB2HeaderSize + 8)) // OutputBufferOffset (header + response base)
	w.WriteUint32(uint32(len(buffer)))        // OutputBufferLength
	w.WriteBytes(buffer)                      // Buffer

	return w.Bytes(), status
}

// queryFileInfo handles file information queries
func (h *SMBHandler) queryFileInfo(of *OpenFile, fileInfoClass uint8) ([]byte, NTStatus) {
	// Get file info
	info, err := of.File.Stat()
	if err != nil {
		h.server.logger.Debug("Stat failed for %s: %v", of.Path, err)
		return nil, STATUS_NO_SUCH_FILE
	}

	attrs := modeToAttributes(info.Mode())

	switch fileInfoClass {
	case FileBasicInformation:
		return h.buildFileBasicInformation(info, attrs), STATUS_SUCCESS

	case FileStandardInformation:
		return h.buildFileStandardInformation(info), STATUS_SUCCESS

	case FileInternalInformation:
		return h.buildFileInternalInformation(of), STATUS_SUCCESS

	case FileEaInformation:
		// EA size - return 0 (no extended attributes)
		w := NewByteWriter(4)
		w.WriteUint32(0) // EaSize
		return w.Bytes(), STATUS_SUCCESS

	case FileAccessInformation:
		w := NewByteWriter(4)
		w.WriteUint32(of.Access) // AccessFlags
		return w.Bytes(), STATUS_SUCCESS

	case FilePositionInformation:
		// Current file position
		pos := int64(0)
		if seeker, ok := of.File.(interface{ Seek(int64, int) (int64, error) }); ok {
			pos, _ = seeker.Seek(0, 1) // SEEK_CUR
		}
		w := NewByteWriter(8)
		w.WriteUint64(uint64(pos)) // CurrentByteOffset
		return w.Bytes(), STATUS_SUCCESS

	case FileAllInformation:
		return h.buildFileAllInformation(of, info, attrs), STATUS_SUCCESS

	case FileNetworkOpenInformation:
		return h.buildFileNetworkOpenInformation(info, attrs), STATUS_SUCCESS

	case FileAttributeTagInformation:
		w := NewByteWriter(8)
		w.WriteUint32(attrs)       // FileAttributes
		w.WriteUint32(0)           // ReparseTag (0 if not a reparse point)
		return w.Bytes(), STATUS_SUCCESS

	default:
		h.server.logger.Debug("Unsupported file info class: %d", fileInfoClass)
		return nil, STATUS_NOT_SUPPORTED
	}
}

// buildFileBasicInformation creates FileBasicInformation response
func (h *SMBHandler) buildFileBasicInformation(info fs.FileInfo, attrs uint32) []byte {
	w := NewByteWriter(40)
	w.WriteUint64(TimeToFiletime(time.Now()))    // CreationTime (use current time)
	w.WriteUint64(TimeToFiletime(info.ModTime())) // LastAccessTime
	w.WriteUint64(TimeToFiletime(info.ModTime())) // LastWriteTime
	w.WriteUint64(TimeToFiletime(info.ModTime())) // ChangeTime
	w.WriteUint32(attrs)                          // FileAttributes
	w.WriteUint32(0)                              // Reserved
	return w.Bytes()
}

// buildFileStandardInformation creates FileStandardInformation response
func (h *SMBHandler) buildFileStandardInformation(info fs.FileInfo) []byte {
	w := NewByteWriter(24)
	allocationSize := uint64(info.Size())
	if allocationSize > 0 {
		// Round up to 4KB allocation units
		allocationSize = ((allocationSize + 4095) / 4096) * 4096
	}
	w.WriteUint64(allocationSize)   // AllocationSize
	w.WriteUint64(uint64(info.Size())) // EndOfFile
	w.WriteUint32(1)                   // NumberOfLinks
	w.WriteOneByte(0)                     // DeletePending
	if info.IsDir() {
		w.WriteOneByte(1)                 // Directory
	} else {
		w.WriteOneByte(0)                 // Directory
	}
	w.WriteUint16(0)                   // Reserved
	return w.Bytes()
}

// buildFileInternalInformation creates FileInternalInformation response
func (h *SMBHandler) buildFileInternalInformation(of *OpenFile) []byte {
	w := NewByteWriter(8)
	// Use volatile file ID as the index number
	w.WriteUint64(of.ID.Volatile) // IndexNumber
	return w.Bytes()
}

// buildFileAllInformation creates FileAllInformation response
func (h *SMBHandler) buildFileAllInformation(of *OpenFile, info fs.FileInfo, attrs uint32) []byte {
	w := NewByteWriter(256)

	// BasicInformation
	w.WriteUint64(TimeToFiletime(time.Now()))    // CreationTime
	w.WriteUint64(TimeToFiletime(info.ModTime())) // LastAccessTime
	w.WriteUint64(TimeToFiletime(info.ModTime())) // LastWriteTime
	w.WriteUint64(TimeToFiletime(info.ModTime())) // ChangeTime
	w.WriteUint32(attrs)                          // FileAttributes
	w.WriteUint32(0)                              // Reserved

	// StandardInformation
	allocationSize := uint64(info.Size())
	if allocationSize > 0 {
		allocationSize = ((allocationSize + 4095) / 4096) * 4096
	}
	w.WriteUint64(allocationSize)   // AllocationSize
	w.WriteUint64(uint64(info.Size())) // EndOfFile
	w.WriteUint32(1)                   // NumberOfLinks
	w.WriteOneByte(0)                     // DeletePending
	if info.IsDir() {
		w.WriteOneByte(1)                 // Directory
	} else {
		w.WriteOneByte(0)                 // Directory
	}
	w.WriteUint16(0)                   // Reserved

	// InternalInformation
	w.WriteUint64(of.ID.Volatile) // IndexNumber

	// EaInformation
	w.WriteUint32(0) // EaSize

	// AccessInformation
	w.WriteUint32(of.Access) // AccessFlags

	// PositionInformation
	pos := int64(0)
	if seeker, ok := of.File.(interface{ Seek(int64, int) (int64, error) }); ok {
		pos, _ = seeker.Seek(0, 1) // SEEK_CUR
	}
	w.WriteUint64(uint64(pos)) // CurrentByteOffset

	// ModeInformation
	w.WriteUint32(0) // Mode

	// AlignmentInformation
	w.WriteUint32(0) // AlignmentRequirement

	// NameInformation
	name := path.Base(of.Path)
	nameBytes := EncodeStringToUTF16LE(name)
	w.WriteUint32(uint32(len(nameBytes))) // FileNameLength
	w.WriteBytes(nameBytes)               // FileName

	return w.Bytes()
}

// buildFileNetworkOpenInformation creates FileNetworkOpenInformation response
func (h *SMBHandler) buildFileNetworkOpenInformation(info fs.FileInfo, attrs uint32) []byte {
	w := NewByteWriter(56)
	allocationSize := uint64(info.Size())
	if allocationSize > 0 {
		allocationSize = ((allocationSize + 4095) / 4096) * 4096
	}
	w.WriteUint64(TimeToFiletime(time.Now()))    // CreationTime
	w.WriteUint64(TimeToFiletime(info.ModTime())) // LastAccessTime
	w.WriteUint64(TimeToFiletime(info.ModTime())) // LastWriteTime
	w.WriteUint64(TimeToFiletime(info.ModTime())) // ChangeTime
	w.WriteUint64(allocationSize)                 // AllocationSize
	w.WriteUint64(uint64(info.Size()))            // EndOfFile
	w.WriteUint32(attrs)                          // FileAttributes
	w.WriteUint32(0)                              // Reserved
	return w.Bytes()
}

// queryFilesystemInfo handles filesystem information queries
func (h *SMBHandler) queryFilesystemInfo(filesystem absfs.FileSystem, fileInfoClass uint8) ([]byte, NTStatus) {
	switch fileInfoClass {
	case FileFsVolumeInformation:
		return h.buildFileFsVolumeInformation(), STATUS_SUCCESS

	case FileFsSizeInformation:
		return h.buildFileFsSizeInformation(), STATUS_SUCCESS

	case FileFsAttributeInformation:
		return h.buildFileFsAttributeInformation(), STATUS_SUCCESS

	case FileFsFullSizeInformation:
		return h.buildFileFsFullSizeInformation(), STATUS_SUCCESS

	default:
		h.server.logger.Debug("Unsupported filesystem info class: %d", fileInfoClass)
		return nil, STATUS_NOT_SUPPORTED
	}
}

// buildFileFsVolumeInformation creates FileFsVolumeInformation response
func (h *SMBHandler) buildFileFsVolumeInformation() []byte {
	volumeLabel := "SMB Share"
	labelBytes := EncodeStringToUTF16LE(volumeLabel)

	w := NewByteWriter(64)
	w.WriteUint64(TimeToFiletime(time.Now())) // VolumeCreationTime
	w.WriteUint32(0x12345678)                 // VolumeSerialNumber (arbitrary)
	w.WriteUint32(uint32(len(labelBytes)))    // VolumeLabelLength
	w.WriteOneByte(0)                            // SupportsObjects
	w.WriteOneByte(0)                            // Reserved
	w.WriteBytes(labelBytes)                  // VolumeLabel
	return w.Bytes()
}

// buildFileFsSizeInformation creates FileFsSizeInformation response
func (h *SMBHandler) buildFileFsSizeInformation() []byte {
	w := NewByteWriter(24)
	// Report 1TB total, 500GB available as defaults
	totalUnits := uint64(1024 * 1024 * 256)    // 1TB in 4KB units
	availableUnits := uint64(1024 * 1024 * 128) // 500GB in 4KB units
	w.WriteUint64(totalUnits)      // TotalAllocationUnits
	w.WriteUint64(availableUnits)  // AvailableAllocationUnits
	w.WriteUint32(8)               // SectorsPerAllocationUnit (4KB = 8 * 512)
	w.WriteUint32(512)             // BytesPerSector
	return w.Bytes()
}

// buildFileFsAttributeInformation creates FileFsAttributeInformation response
func (h *SMBHandler) buildFileFsAttributeInformation() []byte {
	fsName := "SMBFS"
	fsNameBytes := EncodeStringToUTF16LE(fsName)

	// Filesystem attributes
	const (
		FILE_CASE_SENSITIVE_SEARCH        = 0x00000001
		FILE_CASE_PRESERVED_NAMES         = 0x00000002
		FILE_UNICODE_ON_DISK              = 0x00000004
		FILE_PERSISTENT_ACLS              = 0x00000008
		FILE_FILE_COMPRESSION             = 0x00000010
		FILE_VOLUME_QUOTAS                = 0x00000020
		FILE_SUPPORTS_SPARSE_FILES        = 0x00000040
		FILE_SUPPORTS_REPARSE_POINTS      = 0x00000080
		FILE_SUPPORTS_REMOTE_STORAGE      = 0x00000100
		FILE_VOLUME_IS_COMPRESSED         = 0x00008000
		FILE_SUPPORTS_OBJECT_IDS          = 0x00010000
		FILE_SUPPORTS_ENCRYPTION          = 0x00020000
		FILE_NAMED_STREAMS                = 0x00040000
		FILE_READ_ONLY_VOLUME             = 0x00080000
	)

	attrs := uint32(FILE_CASE_PRESERVED_NAMES |
		FILE_UNICODE_ON_DISK |
		FILE_PERSISTENT_ACLS)

	w := NewByteWriter(64)
	w.WriteUint32(attrs)                   // FileSystemAttributes
	w.WriteUint32(255)                     // MaximumComponentNameLength
	w.WriteUint32(uint32(len(fsNameBytes))) // FileSystemNameLength
	w.WriteBytes(fsNameBytes)              // FileSystemName
	return w.Bytes()
}

// buildFileFsFullSizeInformation creates FileFsFullSizeInformation response
func (h *SMBHandler) buildFileFsFullSizeInformation() []byte {
	w := NewByteWriter(32)
	// Report 1TB total, 500GB available as defaults
	totalUnits := uint64(1024 * 1024 * 256)    // 1TB in 4KB units
	availableUnits := uint64(1024 * 1024 * 128) // 500GB in 4KB units
	w.WriteUint64(totalUnits)       // TotalAllocationUnits
	w.WriteUint64(availableUnits)   // CallerAvailableAllocationUnits
	w.WriteUint64(availableUnits)   // ActualAvailableAllocationUnits
	w.WriteUint32(8)                // SectorsPerAllocationUnit (4KB = 8 * 512)
	w.WriteUint32(512)              // BytesPerSector
	return w.Bytes()
}

// handleSetInfo handles SMB2 SET_INFO requests
func (h *SMBHandler) handleSetInfo(state *connState, msg *SMB2Message) ([]byte, NTStatus) {
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

	infoType := r.ReadOneByte()
	fileInfoClass := r.ReadOneByte()
	bufferLength := r.ReadUint32()
	bufferOffset := r.ReadUint16()
	_ = r.ReadUint16() // Reserved
	additionalInfo := r.ReadUint32()
	fileID := r.ReadFileID()

	// Suppress unused variable warnings
	_ = additionalInfo

	h.server.logger.Debug("SET_INFO: type=%d class=%d bufferLen=%d fileID=%v",
		infoType, fileInfoClass, bufferLength, fileID)

	// Get the file handle
	of := tree.Share.fileHandles.GetByTree(fileID, tree.ID, session.ID)
	if of == nil {
		return h.buildErrorResponse(), STATUS_FILE_CLOSED
	}

	// Check if share is read-only
	if tree.IsReadOnly {
		return h.buildErrorResponse(), STATUS_ACCESS_DENIED
	}

	// Update last access time
	tree.Share.fileHandles.UpdateLastAccess(fileID)

	// Get buffer data
	bufferStart := int(bufferOffset) - SMB2HeaderSize
	if bufferStart < 0 || bufferStart+int(bufferLength) > len(msg.Payload) {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}
	buffer := msg.Payload[bufferStart : bufferStart+int(bufferLength)]

	switch infoType {
	case SMB2_0_INFO_FILE:
		status = h.setFileInfo(tree.Share, of, fileInfoClass, buffer)
	case SMB2_0_INFO_FILESYSTEM:
		// Filesystem info is read-only
		return h.buildErrorResponse(), STATUS_NOT_SUPPORTED
	case SMB2_0_INFO_SECURITY:
		// Security info not supported yet
		return h.buildErrorResponse(), STATUS_NOT_SUPPORTED
	case SMB2_0_INFO_QUOTA:
		// Quota info not supported
		return h.buildErrorResponse(), STATUS_NOT_SUPPORTED
	default:
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	if status != STATUS_SUCCESS {
		return h.buildErrorResponse(), status
	}

	// Build response
	w := NewByteWriter(2)
	w.WriteUint16(2) // StructureSize

	return w.Bytes(), STATUS_SUCCESS
}

// setFileInfo handles file information set operations
func (h *SMBHandler) setFileInfo(share *Share, of *OpenFile, fileInfoClass uint8, buffer []byte) NTStatus {
	switch fileInfoClass {
	case FileBasicInformation:
		return h.setFileBasicInformation(of, buffer)

	case FileDispositionInformation:
		return h.setFileDispositionInformation(share, of, buffer)

	case FileRenameInformation:
		return h.setFileRenameInformation(share, of, buffer)

	case FileEndOfFileInformation:
		return h.setFileEndOfFileInformation(of, buffer)

	default:
		h.server.logger.Debug("Unsupported set file info class: %d", fileInfoClass)
		return STATUS_NOT_SUPPORTED
	}
}

// setFileBasicInformation handles FileBasicInformation set
func (h *SMBHandler) setFileBasicInformation(of *OpenFile, buffer []byte) NTStatus {
	if len(buffer) < 40 {
		return STATUS_INVALID_PARAMETER
	}

	r := NewByteReader(buffer)
	creationTime := r.ReadUint64()
	lastAccessTime := r.ReadUint64()
	lastWriteTime := r.ReadUint64()
	changeTime := r.ReadUint64()
	fileAttributes := r.ReadUint32()

	// Suppress unused variables
	_ = creationTime
	_ = lastAccessTime
	_ = changeTime

	// Update modification time if specified
	if lastWriteTime != 0 && lastWriteTime != 0xFFFFFFFFFFFFFFFF {
		modTime := FiletimeToTime(lastWriteTime)
		if chtimer, ok := of.File.(interface{ Chtimes(atime, mtime time.Time) error }); ok {
			if err := chtimer.Chtimes(modTime, modTime); err != nil {
				h.server.logger.Debug("Chtimes failed: %v", err)
				return STATUS_ACCESS_DENIED
			}
		}
	}

	// Update file attributes if specified
	if fileAttributes != 0 && fileAttributes != 0xFFFFFFFF {
		mode := attributesToMode(fileAttributes, of.IsDir)
		if chmoder, ok := of.File.(interface{ Chmod(fs.FileMode) error }); ok {
			if err := chmoder.Chmod(mode); err != nil {
				h.server.logger.Debug("Chmod failed: %v", err)
				// Don't fail on chmod errors as not all filesystems support it
			}
		}
	}

	return STATUS_SUCCESS
}

// setFileDispositionInformation handles FileDispositionInformation set
func (h *SMBHandler) setFileDispositionInformation(share *Share, of *OpenFile, buffer []byte) NTStatus {
	if len(buffer) < 1 {
		return STATUS_INVALID_PARAMETER
	}

	deleteOnClose := buffer[0] != 0

	h.server.logger.Debug("Setting DeleteOnClose=%v for %s", deleteOnClose, of.Path)

	// Set the delete on close flag
	share.fileHandles.SetDeleteOnClose(of.ID, deleteOnClose)

	return STATUS_SUCCESS
}

// setFileRenameInformation handles FileRenameInformation set
func (h *SMBHandler) setFileRenameInformation(share *Share, of *OpenFile, buffer []byte) NTStatus {
	if len(buffer) < 20 {
		return STATUS_INVALID_PARAMETER
	}

	r := NewByteReader(buffer)
	replaceIfExists := r.ReadOneByte()
	_ = r.ReadBytes(7) // Reserved
	_ = r.ReadUint64() // RootDirectory (not used)
	fileNameLength := r.ReadUint32()

	if r.Remaining() < int(fileNameLength) {
		return STATUS_INVALID_PARAMETER
	}

	newName := r.ReadUTF16String(int(fileNameLength))

	h.server.logger.Debug("Renaming %s to %s (replace=%v)", of.Path, newName, replaceIfExists)

	// Convert to filesystem path
	newPath := path.Join(path.Dir(of.Path), newName)

	// Check if target exists
	if _, err := share.fs.Stat(newPath); err == nil {
		if replaceIfExists == 0 {
			return STATUS_OBJECT_NAME_COLLISION
		}
	}

	// Perform rename
	if renamer, ok := share.fs.(interface{ Rename(oldname, newname string) error }); ok {
		if err := renamer.Rename(of.Path, newPath); err != nil {
			h.server.logger.Debug("Rename failed: %v", err)
			if os.IsNotExist(err) {
				return STATUS_OBJECT_NAME_NOT_FOUND
			}
			return STATUS_ACCESS_DENIED
		}

		// Update the file handle path
		of.Path = newPath

		return STATUS_SUCCESS
	}

	return STATUS_NOT_SUPPORTED
}

// setFileEndOfFileInformation handles FileEndOfFileInformation set
func (h *SMBHandler) setFileEndOfFileInformation(of *OpenFile, buffer []byte) NTStatus {
	if len(buffer) < 8 {
		return STATUS_INVALID_PARAMETER
	}

	r := NewByteReader(buffer)
	endOfFile := r.ReadUint64()

	h.server.logger.Debug("Truncating %s to size %d", of.Path, endOfFile)

	// Truncate the file
	if truncater, ok := of.File.(interface{ Truncate(size int64) error }); ok {
		if err := truncater.Truncate(int64(endOfFile)); err != nil {
			h.server.logger.Debug("Truncate failed: %v", err)
			return STATUS_ACCESS_DENIED
		}
		return STATUS_SUCCESS
	}

	return STATUS_NOT_SUPPORTED
}
