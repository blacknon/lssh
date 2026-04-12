package smbfs

// IOCTL control codes
const (
	// File system control codes
	FSCTL_DFS_GET_REFERRALS           uint32 = 0x00060194
	FSCTL_DFS_GET_REFERRALS_EX        uint32 = 0x000601B0
	FSCTL_PIPE_PEEK                   uint32 = 0x0011400C
	FSCTL_PIPE_WAIT                   uint32 = 0x00110018
	FSCTL_PIPE_TRANSCEIVE             uint32 = 0x0011C017
	FSCTL_SRV_COPYCHUNK               uint32 = 0x001440F2
	FSCTL_SRV_ENUMERATE_SNAPSHOTS     uint32 = 0x00144064
	FSCTL_SRV_REQUEST_RESUME_KEY      uint32 = 0x00140078
	FSCTL_SRV_READ_HASH               uint32 = 0x001441BB
	FSCTL_SRV_COPYCHUNK_WRITE         uint32 = 0x001480F2
	FSCTL_LMR_REQUEST_RESILIENCY      uint32 = 0x001401D4
	FSCTL_QUERY_NETWORK_INTERFACE_INFO uint32 = 0x001401FC
	FSCTL_SET_REPARSE_POINT           uint32 = 0x000900A4
	FSCTL_GET_REPARSE_POINT           uint32 = 0x000900A8
	FSCTL_VALIDATE_NEGOTIATE_INFO     uint32 = 0x00140204
)

// handleIOCTL processes IOCTL requests
// IOCTL is used for various control operations on files and named pipes
func (h *SMBHandler) handleIOCTL(state *connState, msg *SMB2Message) ([]byte, NTStatus) {
	// Validate session
	_, status := h.validateSession(msg.Header)
	if status != STATUS_SUCCESS {
		return h.buildErrorResponse(), status
	}

	// Parse IOCTL request
	// Structure (MS-SMB2 2.2.31):
	//   StructureSize (2): Must be 57
	//   Reserved (2)
	//   CtlCode (4): Control code
	//   FileId (16): File handle
	//   InputOffset (4)
	//   InputCount (4)
	//   MaxInputResponse (4)
	//   OutputOffset (4)
	//   OutputCount (4)
	//   MaxOutputResponse (4)
	//   Flags (4)
	//   Reserved2 (4)
	//   Buffer (variable)

	if len(msg.Payload) < 56 {
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	r := NewByteReader(msg.Payload)
	structSize := r.ReadUint16()
	if structSize != 57 {
		h.server.logger.Warn("IOCTL: Invalid structure size: %d", structSize)
		return h.buildErrorResponse(), STATUS_INVALID_PARAMETER
	}

	_ = r.ReadUint16() // Reserved
	ctlCode := r.ReadUint32()

	// Skip FileId for now
	_ = r.ReadUint64() // FileId.Persistent
	_ = r.ReadUint64() // FileId.Volatile

	inputOffset := r.ReadUint32()
	inputCount := r.ReadUint32()
	_ = r.ReadUint32() // MaxInputResponse
	_ = r.ReadUint32() // OutputOffset
	_ = r.ReadUint32() // OutputCount
	maxOutputResp := r.ReadUint32()
	flags := r.ReadUint32()

	h.server.logger.Debug("IOCTL: CtlCode=0x%08x, InputCount=%d, MaxOutputResp=%d, Flags=0x%x",
		ctlCode, inputCount, maxOutputResp, flags)

	// Extract input buffer if present
	var inputBuffer []byte
	if inputCount > 0 && inputOffset > 0 {
		bufStart := int(inputOffset) - SMB2HeaderSize
		if bufStart >= 0 && bufStart+int(inputCount) <= len(msg.Payload) {
			inputBuffer = msg.Payload[bufStart : bufStart+int(inputCount)]
		}
	}

	// Handle specific IOCTL codes
	switch ctlCode {
	case FSCTL_VALIDATE_NEGOTIATE_INFO:
		return h.handleValidateNegotiateInfo(inputBuffer, maxOutputResp)

	case FSCTL_QUERY_NETWORK_INTERFACE_INFO:
		// Network interface query - return NOT_SUPPORTED for now
		// This is used for SMB multichannel
		return h.buildErrorResponse(), STATUS_NOT_SUPPORTED

	case FSCTL_DFS_GET_REFERRALS, FSCTL_DFS_GET_REFERRALS_EX:
		// DFS referrals - return NOT_SUPPORTED (we don't implement DFS)
		return h.buildErrorResponse(), STATUS_NOT_SUPPORTED

	case FSCTL_PIPE_TRANSCEIVE:
		// Named pipe transceive - used for RPC over named pipes
		return h.handlePipeTransceive(state, msg, inputBuffer, maxOutputResp)

	case FSCTL_SRV_REQUEST_RESUME_KEY:
		// Server-side copy resume key request
		return h.buildErrorResponse(), STATUS_NOT_SUPPORTED

	default:
		h.server.logger.Debug("IOCTL: Unsupported control code 0x%08x", ctlCode)
		return h.buildErrorResponse(), STATUS_NOT_SUPPORTED
	}
}

// handleValidateNegotiateInfo handles FSCTL_VALIDATE_NEGOTIATE_INFO
// This is a security feature to prevent downgrade attacks
func (h *SMBHandler) handleValidateNegotiateInfo(input []byte, maxOutput uint32) ([]byte, NTStatus) {
	// For validate negotiate info, we should verify the negotiate parameters match
	// For simplicity, return NOT_SUPPORTED which is allowed per spec
	// A full implementation would validate capabilities, GUID, security mode, and dialects

	h.server.logger.Debug("IOCTL: ValidateNegotiateInfo requested (returning NOT_SUPPORTED)")
	return h.buildErrorResponse(), STATUS_NOT_SUPPORTED
}

// handlePipeTransceive handles FSCTL_PIPE_TRANSCEIVE for named pipe operations
func (h *SMBHandler) handlePipeTransceive(state *connState, msg *SMB2Message, input []byte, maxOutput uint32) ([]byte, NTStatus) {
	// Named pipe transceive is used for RPC calls over SMB
	// For IPC$ share, this is where RPC requests would be processed
	// For now, return NOT_SUPPORTED

	h.server.logger.Debug("IOCTL: PipeTransceive requested (returning NOT_SUPPORTED)")
	return h.buildErrorResponse(), STATUS_NOT_SUPPORTED
}

// buildIOCTLResponse builds an IOCTL response
func (h *SMBHandler) buildIOCTLResponse(ctlCode uint32, fileID [16]byte, output []byte) []byte {
	// IOCTL response structure (MS-SMB2 2.2.32):
	//   StructureSize (2): Must be 49
	//   Reserved (2)
	//   CtlCode (4)
	//   FileId (16)
	//   InputOffset (4)
	//   InputCount (4)
	//   OutputOffset (4)
	//   OutputCount (4)
	//   Flags (4)
	//   Reserved2 (4)
	//   Buffer (variable)

	outputLen := len(output)
	w := NewByteWriter(48 + outputLen)

	w.WriteUint16(49)     // StructureSize
	w.WriteUint16(0)      // Reserved
	w.WriteUint32(ctlCode)
	w.WriteBytes(fileID[:]) // FileId

	if outputLen > 0 {
		outputOffset := SMB2HeaderSize + 48
		w.WriteUint32(0)                    // InputOffset (no input in response)
		w.WriteUint32(0)                    // InputCount
		w.WriteUint32(uint32(outputOffset)) // OutputOffset
		w.WriteUint32(uint32(outputLen))    // OutputCount
	} else {
		w.WriteUint32(0) // InputOffset
		w.WriteUint32(0) // InputCount
		w.WriteUint32(0) // OutputOffset
		w.WriteUint32(0) // OutputCount
	}

	w.WriteUint32(0) // Flags
	w.WriteUint32(0) // Reserved2

	if outputLen > 0 {
		w.WriteBytes(output)
	}

	return w.Bytes()
}
