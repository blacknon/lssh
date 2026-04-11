package smbfs

// SMB2 Session Setup flags
const (
	SMB2_SESSION_FLAG_BINDING uint16 = 0x01 // Session binding (multi-channel)
)

// SMB2 Session Flags (in response)
const (
	SMB2_SESSION_FLAG_IS_GUEST uint16 = 0x0001 // Session is guest
	SMB2_SESSION_FLAG_IS_NULL  uint16 = 0x0002 // Session is null (anonymous)
	SMB2_SESSION_FLAG_ENCRYPT  uint16 = 0x0004 // Session requires encryption
)

// handleSessionSetupImpl implements the SESSION_SETUP command handler
// This establishes an authenticated session with the server
func (h *SMBHandler) handleSessionSetupImpl(state *connState, msg *SMB2Message, respHeader *SMB2Header) ([]byte, NTStatus) {
	// Parse SESSION_SETUP request
	// Structure: MS-SMB2 2.2.5
	// StructureSize (2): Must be 25
	// Flags (1): Binding flags
	// SecurityMode (1): Client security mode
	// Capabilities (4): Client capabilities
	// Channel (4): Reserved/channel
	// SecurityBufferOffset (2): Offset to security blob
	// SecurityBufferLength (2): Length of security blob
	// PreviousSessionId (8): For session reconnect

	if len(msg.Payload) < 24 {
		h.server.logger.Warn("SESSION_SETUP: Invalid payload size: %d", len(msg.Payload))
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	r := NewByteReader(msg.Payload)
	structSize := r.ReadUint16()
	if structSize != 25 {
		h.server.logger.Warn("SESSION_SETUP: Invalid structure size: %d", structSize)
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	flags := r.ReadOneByte()
	securityMode := r.ReadOneByte()
	capabilities := r.ReadUint32()
	channel := r.ReadUint32()
	secBufOffset := r.ReadUint16()
	secBufLen := r.ReadUint16()
	previousSessionID := r.ReadUint64()

	// Log request details
	h.server.logger.Debug("SESSION_SETUP: Flags=0x%02x, SecurityMode=0x%02x, Caps=0x%08x",
		flags, securityMode, capabilities)

	// Extract security buffer from payload
	// Offset is from start of SMB2 header, so subtract header size
	var securityBlob []byte
	if secBufLen > 0 {
		secBufStart := int(secBufOffset) - SMB2HeaderSize
		if secBufStart < 0 || secBufStart+int(secBufLen) > len(msg.Payload) {
			h.server.logger.Warn("SESSION_SETUP: Invalid security buffer offset/length")
			return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
		}
		securityBlob = msg.Payload[secBufStart : secBufStart+int(secBufLen)]
	}

	// Get or create session
	var session *Session
	var isNewSession bool

	// Check if reconnecting to previous session
	if previousSessionID != 0 {
		session = h.server.sessions.GetSession(previousSessionID)
		if session != nil {
			h.server.logger.Debug("SESSION_SETUP: Reconnecting to session %d", previousSessionID)
			isNewSession = false
		}
	}

	// If no existing session (or reconnect failed), check current session ID
	if session == nil && msg.Header.SessionID != 0 {
		session = h.server.sessions.GetSession(msg.Header.SessionID)
		if session != nil {
			h.server.logger.Debug("SESSION_SETUP: Continuing session %d", msg.Header.SessionID)
			isNewSession = false
		}
	}

	// Create new session if needed
	if session == nil {
		h.server.logger.Debug("SESSION_SETUP: Creating new session")
		session = h.server.sessions.CreateSession(
			h.server.options.MaxDialect,
			[16]byte{}, // TODO: Extract client GUID from negotiate context
			state.remoteAddr,
		)
		isNewSession = true
	}

	// Get or create authenticator for this session
	// NTLM requires multiple roundtrips, so we need to persist state
	var authenticator Authenticator
	if session.Authenticator != nil {
		authenticator = session.Authenticator
	} else {
		// Create NTLM authenticator for new sessions
		authenticator = NewNTLMAuthenticator(
			h.server.options.ServerName,
			h.server.options.Users,
			h.server.options.AllowGuest,
		)
		session.Authenticator = authenticator
	}

	// Perform authentication
	authResult, err := authenticator.Authenticate(securityBlob)
	if err != nil {
		h.server.logger.Error("SESSION_SETUP: Authentication error: %v", err)
		// Clean up new session on auth failure
		if isNewSession {
			h.server.sessions.DestroySession(session.ID)
		}
		return h.buildErrorResponse(), STATUS_LOGON_FAILURE
	}

	// Check authentication result
	if !authResult.Success {
		// Check if more processing is required (multi-stage auth like NTLM)
		if authResult.ResponseBlob != nil {
			// Multi-stage authentication - return challenge
			h.server.logger.Debug("SESSION_SETUP: Authentication requires more processing")

			// Update response header with session ID
			respHeader.SessionID = session.ID

			// Build SESSION_SETUP response with security blob
			w := NewByteWriter(128)
			w.WriteUint16(9)                        // StructureSize
			w.WriteUint16(0)                        // SessionFlags (not yet authenticated)
			secBufOffset := SMB2HeaderSize + 8      // After header and fixed response
			w.WriteUint16(uint16(secBufOffset))     // SecurityBufferOffset
			w.WriteUint16(uint16(len(authResult.ResponseBlob))) // SecurityBufferLength
			w.WriteBytes(authResult.ResponseBlob)   // Security buffer

			return w.Bytes(), STATUS_MORE_PROCESSING_REQUIRED
		}

		// Authentication failed completely
		h.server.logger.Warn("SESSION_SETUP: Authentication failed")
		if isNewSession {
			h.server.sessions.DestroySession(session.ID)
		}
		return h.buildErrorResponse(), STATUS_LOGON_FAILURE
	}

	// Authentication succeeded - derive signing key from session key
	var signingKey []byte
	if authResult.SessionKey != nil {
		signingKey = DeriveSigningKey(authResult.SessionKey, state.dialect, state.preauthHash)
		h.server.logger.Debug("SESSION_SETUP: Derived signing key (dialect=%s, keyLen=%d)",
			state.dialect.String(), len(signingKey))
	}

	// Mark session as valid with derived signing key
	session.SetValid(authResult.Username, authResult.Domain, authResult.IsGuest, signingKey)
	state.session = session

	h.server.logger.Info("SESSION_SETUP: Session %d established - User=%s, Guest=%v, Signing=%v",
		session.ID, authResult.Username, authResult.IsGuest, signingKey != nil)

	// Update response header with session ID
	respHeader.SessionID = session.ID

	// Build SESSION_SETUP response
	// Structure: MS-SMB2 2.2.6
	// StructureSize (2): Must be 9
	// SessionFlags (2): Session flags (guest, null, encrypt)
	// SecurityBufferOffset (2): Offset to security blob
	// SecurityBufferLength (2): Length of security blob
	// SecurityBuffer (variable): Security blob (if any)

	w := NewByteWriter(64)
	w.WriteUint16(9) // StructureSize

	// Set session flags
	var sessionFlags uint16
	if authResult.IsGuest {
		sessionFlags |= SMB2_SESSION_FLAG_IS_GUEST
	}
	w.WriteUint16(sessionFlags)

	// Security buffer (for response blob from authenticator)
	if authResult.ResponseBlob != nil && len(authResult.ResponseBlob) > 0 {
		secBufOffset := SMB2HeaderSize + 8 // After header and fixed response
		w.WriteUint16(uint16(secBufOffset))
		w.WriteUint16(uint16(len(authResult.ResponseBlob)))
		w.WriteBytes(authResult.ResponseBlob)
	} else {
		// No security response blob
		w.WriteUint16(0) // SecurityBufferOffset
		w.WriteUint16(0) // SecurityBufferLength
	}

	// Suppress unused variable warnings
	_ = channel

	return w.Bytes(), STATUS_SUCCESS
}

// handleLogoffImpl implements the LOGOFF command handler
// This destroys the session and releases all associated resources
func (h *SMBHandler) handleLogoffImpl(state *connState, msg *SMB2Message) ([]byte, NTStatus) {
	// Validate session exists
	session, status := h.validateSession(msg.Header)
	if status != STATUS_SUCCESS {
		h.server.logger.Warn("LOGOFF: Invalid session %d", msg.Header.SessionID)
		return h.buildErrorResponse(), status
	}

	// Parse LOGOFF request (minimal - just structure size)
	// Structure: MS-SMB2 2.2.7
	// StructureSize (2): Must be 4
	// Reserved (2): Reserved

	if len(msg.Payload) < 4 {
		h.server.logger.Warn("LOGOFF: Invalid payload size: %d", len(msg.Payload))
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	r := NewByteReader(msg.Payload)
	structSize := r.ReadUint16()
	if structSize != 4 {
		h.server.logger.Warn("LOGOFF: Invalid structure size: %d", structSize)
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	h.server.logger.Info("LOGOFF: Session %d (User=%s)", session.ID, session.Username)

	// Get all tree connections before destroying session
	trees := session.GetAllTreeConnections()

	// Clean up file handles for each tree connection
	for _, tree := range trees {
		// Release all file handles for this tree/session combination
		if tree.Share != nil {
			h.server.logger.Debug("LOGOFF: Releasing file handles for tree %d (share=%s)",
				tree.ID, tree.ShareName)
			tree.Share.fileHandles.ReleaseByTree(tree.ID, session.ID)
		}
	}

	// Destroy the session (this also removes all tree connections)
	h.server.sessions.DestroySession(session.ID)

	// Clear session from connection state
	state.session = nil

	// Build LOGOFF response
	// Structure: MS-SMB2 2.2.8
	// StructureSize (2): Must be 4
	// Reserved (2): Reserved

	w := NewByteWriter(4)
	w.WriteUint16(4) // StructureSize
	w.WriteUint16(0) // Reserved

	return w.Bytes(), STATUS_SUCCESS
}
