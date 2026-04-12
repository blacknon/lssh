package smbfs

// SMB2 TREE_CONNECT and TREE_DISCONNECT handlers
// These handlers manage tree connections to shares within a session

// handleTreeConnectImpl implements TREE_CONNECT command
// Request structure (9 bytes fixed):
//
//	StructureSize (2 bytes): Must be 9
//	Flags (2 bytes): Reserved/flags
//	PathOffset (2 bytes): Offset to path from start of SMB2 header
//	PathLength (2 bytes): Length of path in bytes
//	Buffer (variable): UTF-16LE encoded UNC path (\\server\share)
//
// Response structure (16 bytes):
//
//	StructureSize (2 bytes): Must be 16
//	ShareType (1 byte): Type of share (DISK, PIPE, PRINT)
//	Reserved (1 byte): Reserved
//	ShareFlags (4 bytes): Properties of the share
//	Capabilities (4 bytes): Capabilities of the share
//	MaximalAccess (4 bytes): Maximal access rights for the user
func (h *SMBHandler) handleTreeConnectImpl(state *connState, msg *SMB2Message, respHeader *SMB2Header) ([]byte, NTStatus) {
	// Validate session exists and is valid
	session, status := h.validateSession(msg.Header)
	if status != STATUS_SUCCESS {
		return h.buildErrorResponse(), status
	}

	// Parse request - minimum 8 bytes (structSize + flags + pathOffset + pathLen)
	if len(msg.Payload) < 8 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	r := NewByteReader(msg.Payload)
	structSize := r.ReadUint16()
	if structSize != 9 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	_ = r.ReadUint16()           // Flags/Reserved
	pathOffset := r.ReadUint16() // Offset from start of SMB2 header
	pathLen := r.ReadUint16()

	// Extract share path from UTF-16LE encoded buffer
	// Path offset is from the start of the SMB2 header
	pathStart := int(pathOffset) - SMB2HeaderSize
	if pathStart < 0 || pathStart+int(pathLen) > len(msg.Payload) {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}
	path := DecodeUTF16LEToString(msg.Payload[pathStart : pathStart+int(pathLen)])

	h.server.logger.Debug("TREE_CONNECT path: %s", path)

	// Parse share name from path using extractShareName helper
	// Expected format: \\server\sharename
	shareName := extractShareName(path)
	if shareName == "" {
		return h.buildErrorResponse(), STATUS_BAD_NETWORK_NAME
	}

	// Look up share via server.GetShare(shareName)
	share := h.server.GetShare(shareName)
	if share == nil {
		h.server.logger.Warn("Share not found: %s", shareName)
		return h.buildErrorResponse(), STATUS_BAD_NETWORK_NAME
	}

	// Check user access via share.CheckUserAccess()
	if !share.CheckUserAccess(session.Username, session.IsGuest) {
		return h.buildErrorResponse(), STATUS_ACCESS_DENIED
	}

	// Create tree connection via session.AddTreeConnection()
	tree := session.AddTreeConnection(shareName, share, share.IsReadOnly())

	// Set response header TreeID
	respHeader.TreeID = tree.ID

	h.server.logger.Info("Tree connected: ID=%d, Share=%s, User=%s", tree.ID, shareName, session.Username)

	// Build response (structure size 16)
	w := NewByteWriter(16)
	w.WriteUint16(16)                           // StructureSize
	w.WriteOneByte(uint8(share.GetShareType())) // ShareType from share config
	w.WriteOneByte(0)                           // Reserved

	// ShareFlags - manual caching (most compatible)
	// Bits 0-3: caching policy (0 = manual caching of documents)
	shareFlags := uint32(0)
	w.WriteUint32(shareFlags)

	// Capabilities - don't claim DFS since we don't support it
	capabilities := uint32(0)
	w.WriteUint32(capabilities)

	// MaximalAccess - use specific access rights, not MAXIMUM_ALLOWED
	// MAXIMUM_ALLOWED (0x02000000) is a request flag, not appropriate in response
	var maximalAccess uint32
	if share.IsReadOnly() {
		// Read-only access
		maximalAccess = FILE_READ_DATA | FILE_READ_ATTRIBUTES | FILE_READ_EA | READ_CONTROL | SYNCHRONIZE
	} else {
		// Full access - standard file access rights
		maximalAccess = FILE_READ_DATA | FILE_WRITE_DATA | FILE_APPEND_DATA |
			FILE_READ_EA | FILE_WRITE_EA |
			FILE_EXECUTE | FILE_DELETE_CHILD |
			FILE_READ_ATTRIBUTES | FILE_WRITE_ATTRIBUTES |
			DELETE | READ_CONTROL | WRITE_DAC | WRITE_OWNER | SYNCHRONIZE
	}
	w.WriteUint32(maximalAccess)

	return w.Bytes(), STATUS_SUCCESS
}

// handleTreeDisconnectImpl implements TREE_DISCONNECT command
// Request structure (4 bytes):
//
//	StructureSize (2 bytes): Must be 4
//	Reserved (2 bytes): Reserved
//
// Response structure (4 bytes):
//
//	StructureSize (2 bytes): Must be 4
//	Reserved (2 bytes): Reserved
func (h *SMBHandler) handleTreeDisconnectImpl(state *connState, msg *SMB2Message) ([]byte, NTStatus) {
	// Validate session and tree connection
	session, tree, status := h.validateTree(msg.Header)
	if status != STATUS_SUCCESS {
		return h.buildErrorResponse(), status
	}

	h.server.logger.Debug("TREE_DISCONNECT: TreeID=%d, Share=%s", tree.ID, tree.ShareName)

	// Release all file handles for this tree via share.fileHandles.ReleaseByTree()
	tree.Share.fileHandles.ReleaseByTree(tree.ID, session.ID)

	// Remove tree connection via session.RemoveTreeConnection()
	session.RemoveTreeConnection(tree.ID)

	h.server.logger.Info("Tree disconnected: ID=%d, Share=%s", tree.ID, tree.ShareName)

	// Build simple response (structure size 4)
	w := NewByteWriter(4)
	w.WriteUint16(4) // StructureSize
	w.WriteUint16(0) // Reserved

	return w.Bytes(), STATUS_SUCCESS
}
