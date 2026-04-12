package smbfs

// handleNegotiateImpl handles the SMB2 NEGOTIATE request
// This is the first message in the SMB2 protocol handshake where the client
// and server agree on the SMB dialect version to use.
//
// Request structure (36 bytes minimum):
// - StructureSize (2): Must be 36
// - DialectCount (2): Number of dialect revisions in array
// - SecurityMode (2): Client security mode flags
// - Reserved (2): Must be zero
// - Capabilities (4): Client capability flags
// - ClientGUID (16): Client GUID (optional, can be zero)
// - NegotiateContextOffset (4): Offset to negotiate contexts (SMB 3.1.1+)
// - NegotiateContextCount (2): Number of negotiate contexts
// - Reserved2 (2): Must be zero
// - Dialects (2 * DialectCount): Array of dialect revision codes
//
// Response structure (65 bytes minimum):
// - StructureSize (2): Must be 65
// - SecurityMode (2): Server security mode flags
// - DialectRevision (2): Selected dialect
// - NegotiateContextCount (2): Number of negotiate contexts (SMB 3.1.1+)
// - ServerGUID (16): Server GUID
// - Capabilities (4): Server capability flags
// - MaxTransactSize (4): Maximum transaction size
// - MaxReadSize (4): Maximum read size
// - MaxWriteSize (4): Maximum write size
// - SystemTime (8): Current system time
// - ServerStartTime (8): Server start time
// - SecurityBufferOffset (2): Offset to security buffer
// - SecurityBufferLength (2): Length of security buffer
// - NegotiateContextOffset (4): Offset to negotiate contexts (SMB 3.1.1+)
// - SecurityBuffer: SPNEGO token (optional)
func (h *SMBHandler) handleNegotiateImpl(state *connState, msg *SMB2Message) ([]byte, NTStatus) {
	opts := h.server.options

	// Handle SMB1 client upgrade
	// If payload is empty, this is from handleSMB1Negotiate
	if len(msg.Payload) == 0 {
		return h.buildNegotiateResponse(opts.MaxDialect, [16]byte{}, 0, 0), STATUS_SUCCESS
	}

	// Parse request
	if len(msg.Payload) < 36 {
		h.server.logger.Warn("NEGOTIATE request too short: %d bytes", len(msg.Payload))
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	r := NewByteReader(msg.Payload)

	// Validate structure size
	structSize := r.ReadUint16()
	if structSize != 36 {
		h.server.logger.Warn("Invalid NEGOTIATE StructureSize: %d (expected 36)", structSize)
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	// Read request fields
	dialectCount := r.ReadUint16()
	clientSecurityMode := r.ReadUint16()
	_ = r.ReadUint16() // Reserved
	clientCapabilities := r.ReadUint32()
	clientGUID := r.ReadGUID()
	negContextOffset := r.ReadUint32() // SMB 3.1.1+ only
	negContextCount := r.ReadUint16()  // SMB 3.1.1+ only
	_ = r.ReadUint16()                 // Reserved2

	// Read dialect array
	if r.Remaining() < int(dialectCount)*2 {
		h.server.logger.Warn("NEGOTIATE: not enough data for dialect array")
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	clientDialects := make([]SMBDialect, dialectCount)
	for i := uint16(0); i < dialectCount; i++ {
		clientDialects[i] = SMBDialect(r.ReadUint16())
	}

	// Log the negotiation
	h.server.logger.Debug("NEGOTIATE: Client dialects=%v, SecurityMode=0x%04x, Capabilities=0x%08x, GUID=%s",
		formatDialects(clientDialects), clientSecurityMode, clientCapabilities, GUIDToString(clientGUID))

	// Select the highest common dialect
	selectedDialect := h.selectDialect(clientDialects)
	if selectedDialect == 0 {
		h.server.logger.Warn("NEGOTIATE: No common dialect found")
		return h.buildErrorResponse(), STATUS_NOT_SUPPORTED
	}

	// Check if selected dialect meets minimum requirement
	if selectedDialect < opts.MinDialect {
		h.server.logger.Warn("NEGOTIATE: Selected dialect %s below minimum %s",
			selectedDialect.String(), opts.MinDialect.String())
		return h.buildErrorResponse(), STATUS_NOT_SUPPORTED
	}

	h.server.logger.Info("NEGOTIATE: Selected dialect %s", selectedDialect.String())

	// Store negotiation state
	state.session = nil // Clear any previous session
	state.dialect = selectedDialect

	// Check if signing is required
	// Client security mode bit 0x02 = signing required
	clientSigningRequired := clientSecurityMode&0x02 != 0
	serverSigningRequired := h.server.options.SigningRequired

	// Signing is required if either client or server requires it
	state.signingRequired = clientSigningRequired || serverSigningRequired

	h.server.logger.Debug("NEGOTIATE: Signing - client required=%v, server required=%v, negotiated required=%v",
		clientSigningRequired, serverSigningRequired, state.signingRequired)

	// Always log negotiate context info for debugging
	h.server.logger.Debug("NEGOTIATE: negContextOffset=%d, negContextCount=%d, dialect=%s, payloadLen=%d, rawLen=%d",
		negContextOffset, negContextCount, selectedDialect.String(), len(msg.Payload), len(msg.RawBytes))

	// For SMB 3.1.1, parse and log client negotiate contexts
	if selectedDialect >= SMB3_1_1 && negContextCount > 0 {
		h.parseClientNegotiateContexts(msg.RawBytes, negContextOffset, negContextCount)
	}

	// Build and return response
	return h.buildNegotiateResponse(selectedDialect, clientGUID, negContextOffset, negContextCount), STATUS_SUCCESS
}

// selectDialect chooses the highest common dialect between client and server
func (h *SMBHandler) selectDialect(clientDialects []SMBDialect) SMBDialect {
	opts := h.server.options

	// Try each server dialect from highest to lowest
	for _, serverDialect := range SupportedDialects {
		// Skip if above max dialect
		if serverDialect > opts.MaxDialect {
			continue
		}

		// Skip if below min dialect
		if serverDialect < opts.MinDialect {
			continue
		}

		// Check if client supports this dialect
		for _, clientDialect := range clientDialects {
			if clientDialect == serverDialect {
				return serverDialect
			}
		}
	}

	return 0 // No common dialect
}

// SMB 3.1.1 Negotiate Context Types
const (
	SMB2_PREAUTH_INTEGRITY_CAPABILITIES uint16 = 0x0001
	SMB2_ENCRYPTION_CAPABILITIES        uint16 = 0x0002
	SMB2_SIGNING_CAPABILITIES           uint16 = 0x0008
)

// SMB 3.1.1 Hash Algorithms
const (
	SMB2_PREAUTH_INTEGRITY_SHA512 uint16 = 0x0001
)

// SMB 3.1.1 Cipher IDs
const (
	SMB2_ENCRYPTION_AES128_CCM uint16 = 0x0001
	SMB2_ENCRYPTION_AES128_GCM uint16 = 0x0002
)

// buildNegotiateResponse constructs the SMB2 NEGOTIATE response
func (h *SMBHandler) buildNegotiateResponse(dialect SMBDialect, clientGUID [16]byte, negContextOffset uint32, negContextCount uint16) []byte {
	opts := h.server.options

	// Determine security mode
	securityMode := SMB2_NEGOTIATE_SIGNING_ENABLED
	if opts.SigningRequired {
		securityMode |= SMB2_NEGOTIATE_SIGNING_REQUIRED
	}

	// Determine capabilities
	// Always advertise LARGE_MTU for better performance
	capabilities := SMB2_GLOBAL_CAP_LARGE_MTU

	// Add DFS capability if we support it
	capabilities |= SMB2_GLOBAL_CAP_DFS

	// Add encryption capability for SMB 3.0+
	if dialect >= SMB3_0 {
		capabilities |= SMB2_GLOBAL_CAP_ENCRYPTION
	}

	// Add multi-channel and persistent handles for SMB 3.0+
	if dialect >= SMB3_0 {
		capabilities |= SMB2_GLOBAL_CAP_MULTI_CHANNEL
		capabilities |= SMB2_GLOBAL_CAP_PERSISTENT_HANDLES
	}

	// Add directory leasing for SMB 3.0.2+
	if dialect >= SMB3_0_2 {
		capabilities |= SMB2_GLOBAL_CAP_DIRECTORY_LEASING
	}

	// Calculate current time and server start time
	systemTime := TimeToFiletime(now())
	serverStartTime := systemTime // For now, use current time as start time

	// Build negotiate contexts for SMB 3.1.1
	var negotiateContexts []byte
	var contextCount uint16
	if dialect >= SMB3_1_1 {
		negotiateContexts, contextCount = h.buildNegotiateContexts()
	}

	// Build response
	w := NewByteWriter(256)
	w.WriteUint16(65)              // StructureSize
	w.WriteUint16(securityMode)    // SecurityMode
	w.WriteUint16(uint16(dialect)) // DialectRevision
	w.WriteUint16(contextCount)    // NegotiateContextCount (or Reserved for < SMB 3.1.1)
	w.WriteGUID(opts.ServerGUID)   // ServerGUID
	w.WriteUint32(capabilities)    // Capabilities
	w.WriteUint32(MaxTransactSize) // MaxTransactSize
	w.WriteUint32(opts.MaxReadSize)  // MaxReadSize
	w.WriteUint32(opts.MaxWriteSize) // MaxWriteSize
	w.WriteUint64(systemTime)        // SystemTime
	w.WriteUint64(serverStartTime)   // ServerStartTime

	// Security buffer (SPNEGO token)
	// For now, we'll use an empty security buffer
	securityBufferOffset := SMB2HeaderSize + 64 // After header + negotiate response base
	securityBufferLength := 0

	w.WriteUint16(uint16(securityBufferOffset)) // SecurityBufferOffset
	w.WriteUint16(uint16(securityBufferLength)) // SecurityBufferLength

	// NegotiateContextOffset - points to after the fixed response
	// Offset is from start of SMB2 header
	if dialect >= SMB3_1_1 && contextCount > 0 {
		// Offset = SMB2 header (64) + fixed response (64) = 128
		// But we need 8-byte alignment, and there's no security buffer
		negContextOff := SMB2HeaderSize + 64 // No padding needed, already aligned
		w.WriteUint32(uint32(negContextOff))
	} else {
		w.WriteUint32(0) // NegotiateContextOffset (or Reserved2)
	}

	// Append negotiate contexts for SMB 3.1.1
	if dialect >= SMB3_1_1 && len(negotiateContexts) > 0 {
		w.WriteBytes(negotiateContexts)
	}

	h.server.logger.Debug("NEGOTIATE response: Dialect=%s, Caps=0x%08x, SecMode=0x%04x",
		dialect.String(), capabilities, securityMode)

	return w.Bytes()
}

// buildNegotiateContexts builds the SMB 3.1.1 negotiate contexts
func (h *SMBHandler) buildNegotiateContexts() ([]byte, uint16) {
	w := NewByteWriter(64)

	// Context 1: SMB2_PREAUTH_INTEGRITY_CAPABILITIES
	// ContextType (2) + DataLength (2) + Reserved (4) + Data
	w.WriteUint16(SMB2_PREAUTH_INTEGRITY_CAPABILITIES) // ContextType
	preauthData := h.buildPreauthIntegrityContext()
	w.WriteUint16(uint16(len(preauthData))) // DataLength
	w.WriteUint32(0)                        // Reserved
	w.WriteBytes(preauthData)

	// Pad to 8-byte boundary before next context
	padding := (8 - (len(preauthData) % 8)) % 8
	for i := 0; i < padding; i++ {
		w.WriteOneByte(0)
	}

	// Context 2: SMB2_ENCRYPTION_CAPABILITIES
	w.WriteUint16(SMB2_ENCRYPTION_CAPABILITIES) // ContextType
	encryptData := h.buildEncryptionContext()
	w.WriteUint16(uint16(len(encryptData))) // DataLength
	w.WriteUint32(0)                        // Reserved
	w.WriteBytes(encryptData)

	// Pad to 8-byte boundary before next context
	encPadding := (8 - (len(encryptData) % 8)) % 8
	for i := 0; i < encPadding; i++ {
		w.WriteOneByte(0)
	}

	// Context 3: SMB2_SIGNING_CAPABILITIES (required by Windows 11 24H2)
	w.WriteUint16(SMB2_SIGNING_CAPABILITIES) // ContextType
	signData := h.buildSigningCapabilitiesContext()
	w.WriteUint16(uint16(len(signData))) // DataLength
	w.WriteUint32(0)                     // Reserved
	w.WriteBytes(signData)

	return w.Bytes(), 3 // Three contexts
}

// buildPreauthIntegrityContext builds the preauth integrity capabilities context
func (h *SMBHandler) buildPreauthIntegrityContext() []byte {
	w := NewByteWriter(32)

	// HashAlgorithmCount (2): Number of hash algorithms
	w.WriteUint16(1)

	// SaltLength (2): Length of salt (we'll use 32 bytes)
	saltLength := uint16(32)
	w.WriteUint16(saltLength)

	// HashAlgorithms (2 * count): SHA-512 only
	w.WriteUint16(SMB2_PREAUTH_INTEGRITY_SHA512)

	// Salt (variable): Random salt
	salt := make([]byte, saltLength)
	// Use a fixed salt for now (in production, use crypto/rand)
	for i := range salt {
		salt[i] = byte(i)
	}
	w.WriteBytes(salt)

	return w.Bytes()
}

// buildEncryptionContext builds the encryption capabilities context
// Per MS-SMB2, server MUST return exactly ONE cipher in the response
func (h *SMBHandler) buildEncryptionContext() []byte {
	w := NewByteWriter(4)

	// CipherCount (2): Server responds with exactly 1 selected cipher
	w.WriteUint16(1)

	// Ciphers (2 * count): AES-128-GCM (preferred)
	w.WriteUint16(SMB2_ENCRYPTION_AES128_GCM)

	return w.Bytes()
}

// SMB 3.1.1 Signing Algorithm IDs
const (
	SMB2_SIGNING_AES_CMAC    uint16 = 0x0001
	SMB2_SIGNING_AES_GMAC    uint16 = 0x0002
	SMB2_SIGNING_HMAC_SHA256 uint16 = 0x0000 // For SMB 3.0.x compatibility
)

// buildSigningCapabilitiesContext builds the signing capabilities context
// Per MS-SMB2, server responds with exactly 1 signing algorithm
func (h *SMBHandler) buildSigningCapabilitiesContext() []byte {
	w := NewByteWriter(4)

	// SigningAlgorithmCount (2): Server responds with exactly 1 selected algorithm
	w.WriteUint16(1)

	// SigningAlgorithms (2 * count): AES-CMAC (required for SMB 3.1.1)
	w.WriteUint16(SMB2_SIGNING_AES_CMAC)

	return w.Bytes()
}

// parseClientNegotiateContexts parses and logs client negotiate contexts for debugging
func (h *SMBHandler) parseClientNegotiateContexts(rawBytes []byte, offset uint32, count uint16) {
	// Offset is from start of SMB2 header in the raw message
	// rawBytes includes NetBIOS header (4 bytes) + SMB2 header (64 bytes) + payload
	// So we need to offset by 4 (NetBIOS) to get to SMB2 header start

	// The offset is from the start of the SMB2 header
	// Skip NetBIOS header (4 bytes) if present
	startOffset := int(offset)
	if len(rawBytes) > 4 && rawBytes[4] == 0xFE && rawBytes[5] == 'S' {
		// NetBIOS header present
		startOffset += 4
	}

	if startOffset >= len(rawBytes) {
		h.server.logger.Debug("NEGOTIATE: Context offset %d beyond message length %d", startOffset, len(rawBytes))
		return
	}

	h.server.logger.Debug("NEGOTIATE: Parsing %d client contexts at offset %d (adjusted=%d)", count, offset, startOffset)

	pos := startOffset
	for i := uint16(0); i < count && pos+8 <= len(rawBytes); i++ {
		contextType := uint16(rawBytes[pos]) | uint16(rawBytes[pos+1])<<8
		dataLen := uint16(rawBytes[pos+2]) | uint16(rawBytes[pos+3])<<8
		// Reserved 4 bytes at pos+4

		contextTypeName := "Unknown"
		switch contextType {
		case SMB2_PREAUTH_INTEGRITY_CAPABILITIES:
			contextTypeName = "PREAUTH_INTEGRITY"
		case SMB2_ENCRYPTION_CAPABILITIES:
			contextTypeName = "ENCRYPTION"
		case 0x0003:
			contextTypeName = "COMPRESSION"
		case 0x0005:
			contextTypeName = "NETNAME_NEGOTIATE"
		case 0x0006:
			contextTypeName = "TRANSPORT_CAPABILITIES"
		case 0x0007:
			contextTypeName = "RDMA_TRANSFORM"
		case 0x0008:
			contextTypeName = "SIGNING_CAPABILITIES"
		}

		h.server.logger.Debug("NEGOTIATE: Context[%d] Type=0x%04x (%s), DataLen=%d",
			i, contextType, contextTypeName, dataLen)

		// Move to next context (8 byte header + data + padding to 8-byte boundary)
		pos += 8 + int(dataLen)
		padding := (8 - (int(dataLen) % 8)) % 8
		pos += padding
	}
}

// formatDialects formats a slice of dialects for logging
func formatDialects(dialects []SMBDialect) string {
	if len(dialects) == 0 {
		return "[]"
	}

	result := "["
	for i, d := range dialects {
		if i > 0 {
			result += ", "
		}
		result += d.String()
	}
	result += "]"
	return result
}
