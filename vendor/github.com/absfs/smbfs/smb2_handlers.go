package smbfs

import (
	"fmt"
	"time"
)

// SMBHandler routes SMB2/3 messages to appropriate handlers
type SMBHandler struct {
	server *Server
}

// NewSMBHandler creates a new SMB handler
func NewSMBHandler(server *Server) *SMBHandler {
	return &SMBHandler{
		server: server,
	}
}

// HandleMessage routes an incoming message to the appropriate handler
func (h *SMBHandler) HandleMessage(state *connState, msg *SMB2Message) (*SMB2Message, error) {
	header := msg.Header
	cmd := header.Command

	h.server.logger.Debug("Received %s (MsgID: %d, SessID: %d, TreeID: %d)",
		CommandName(cmd), header.MessageID, header.SessionID, header.TreeID)

	// Calculate credits to grant (be generous to avoid client stalls)
	creditsToGrant := header.CreditRequest
	if creditsToGrant < 1 {
		creditsToGrant = 1
	}
	if creditsToGrant < 10 {
		creditsToGrant = 10 // Always grant at least 10 credits
	}

	// Build response header
	respHeader := &SMB2Header{
		StructureSize: SMB2HeaderSize,
		Command:       cmd,
		Flags:         SMB2_FLAGS_SERVER_TO_REDIR,
		MessageID:     header.MessageID,
		SessionID:     header.SessionID,
		TreeID:        header.TreeID,
		CreditRequest: creditsToGrant, // Grant generous credits
	}
	copy(respHeader.ProtocolID[:], SMB2ProtocolID)

	// Route to handler
	var payload []byte
	var status NTStatus

	h.server.logger.Debug("Received command: %s (0x%04x), MsgID=%d, SessionID=%d, TreeID=%d",
		CommandName(cmd), cmd, header.MessageID, header.SessionID, header.TreeID)

	switch cmd {
	case SMB2_NEGOTIATE:
		payload, status = h.handleNegotiate(state, msg)

	case SMB2_SESSION_SETUP:
		payload, status = h.handleSessionSetup(state, msg, respHeader)

	case SMB2_LOGOFF:
		payload, status = h.handleLogoff(state, msg)

	case SMB2_TREE_CONNECT:
		payload, status = h.handleTreeConnect(state, msg, respHeader)

	case SMB2_TREE_DISCONNECT:
		payload, status = h.handleTreeDisconnect(state, msg)

	case SMB2_CREATE:
		payload, status = h.handleCreate(state, msg, respHeader)

	case SMB2_CLOSE:
		payload, status = h.handleClose(state, msg)

	case SMB2_READ:
		payload, status = h.handleRead(state, msg)

	case SMB2_WRITE:
		payload, status = h.handleWrite(state, msg)

	case SMB2_FLUSH:
		payload, status = h.handleFlush(state, msg)

	case SMB2_QUERY_DIRECTORY:
		payload, status = h.handleQueryDirectory(state, msg)

	case SMB2_QUERY_INFO:
		payload, status = h.handleQueryInfo(state, msg)

	case SMB2_SET_INFO:
		payload, status = h.handleSetInfo(state, msg)

	case SMB2_ECHO:
		payload, status = h.handleEcho(state, msg)

	case SMB2_CANCEL:
		// CANCEL doesn't get a response
		return nil, nil

	case SMB2_IOCTL:
		payload, status = h.handleIOCTL(state, msg)

	default:
		h.server.logger.Warn("Unsupported command: %s (0x%04x)", CommandName(cmd), cmd)
		status = STATUS_NOT_SUPPORTED
		payload = h.buildErrorResponse()
	}

	respHeader.Status = status

	response := &SMB2Message{
		Header:  respHeader,
		Payload: payload,
	}

	// Check if message should be signed
	// Sign if: session is valid AND has signing key AND (signing required OR request was signed)
	shouldSign := false
	var signingKey []byte

	if state.session != nil && state.session.SigningKey != nil {
		signingKey = state.session.SigningKey
		// Sign if signing is required, or if the incoming message was signed
		requestSigned := header.Flags&SMB2_FLAGS_SIGNED != 0
		shouldSign = state.signingRequired || requestSigned

		// Don't sign certain messages
		// SESSION_SETUP success with STATUS_SUCCESS should be signed (not MORE_PROCESSING_REQUIRED)
		if cmd == SMB2_SESSION_SETUP && status != STATUS_SUCCESS {
			shouldSign = false
		}
		// NEGOTIATE is never signed
		if cmd == SMB2_NEGOTIATE {
			shouldSign = false
		}
	}

	if shouldSign {
		// Set the signed flag in the response header
		respHeader.Flags |= SMB2_FLAGS_SIGNED
		// Signature will be applied when marshaling in writeMessage
		response.SigningKey = signingKey
		response.Dialect = state.dialect
		h.server.logger.Debug("Response will be signed (cmd=%s, dialect=%s)",
			CommandName(cmd), state.dialect.String())
	}

	h.server.logger.Debug("Responding %s status=%s (%d bytes, signed=%v)",
		CommandName(cmd), status.String(), len(payload), shouldSign)

	return response, nil
}

// validateSession validates the session for commands that require it
func (h *SMBHandler) validateSession(header *SMB2Header) (*Session, NTStatus) {
	session, status := h.server.sessions.ValidateSession(header.SessionID)
	if status != STATUS_SUCCESS {
		return nil, status
	}
	return session, STATUS_SUCCESS
}

// validateTree validates both session and tree connection
func (h *SMBHandler) validateTree(header *SMB2Header) (*Session, *TreeConnection, NTStatus) {
	session, status := h.validateSession(header)
	if status != STATUS_SUCCESS {
		return nil, nil, status
	}

	tree, status := session.ValidateTreeConnection(header.TreeID)
	if status != STATUS_SUCCESS {
		return nil, nil, status
	}

	return session, tree, STATUS_SUCCESS
}

// buildErrorResponse creates an empty error response payload
func (h *SMBHandler) buildErrorResponse() []byte {
	w := NewByteWriter(9)
	w.WriteUint16(9) // StructureSize
	w.WriteOneByte(0)   // ErrorContextCount
	w.WriteOneByte(0)   // Reserved
	w.WriteUint32(0) // ByteCount
	w.WriteOneByte(0)   // ErrorData (1 byte for structure)
	return w.Bytes()
}

// Handler forwarders - route to implementations in separate files

func (h *SMBHandler) handleNegotiate(state *connState, msg *SMB2Message) ([]byte, NTStatus) {
	return h.handleNegotiateImpl(state, msg) // smb2_negotiate.go
}

func (h *SMBHandler) handleSessionSetup(state *connState, msg *SMB2Message, respHeader *SMB2Header) ([]byte, NTStatus) {
	return h.handleSessionSetupImpl(state, msg, respHeader) // smb2_session.go
}

func (h *SMBHandler) handleLogoff(state *connState, msg *SMB2Message) ([]byte, NTStatus) {
	return h.handleLogoffImpl(state, msg) // smb2_session.go
}

func (h *SMBHandler) handleTreeConnect(state *connState, msg *SMB2Message, respHeader *SMB2Header) ([]byte, NTStatus) {
	return h.handleTreeConnectImpl(state, msg, respHeader) // smb2_tree.go
}

func (h *SMBHandler) handleTreeDisconnect(state *connState, msg *SMB2Message) ([]byte, NTStatus) {
	return h.handleTreeDisconnectImpl(state, msg) // smb2_tree.go
}

// handleCreate, handleClose, handleRead, handleWrite, handleFlush -> smb2_file.go
// handleQueryDirectory -> smb2_dir.go
// handleQueryInfo, handleSetInfo -> smb2_info.go

func (h *SMBHandler) handleEcho(state *connState, msg *SMB2Message) ([]byte, NTStatus) {
	// ECHO response is simple - just return empty response
	w := NewByteWriter(4)
	w.WriteUint16(4) // StructureSize
	w.WriteUint16(0) // Reserved
	return w.Bytes(), STATUS_SUCCESS
}

// Helper functions

// extractShareName extracts the share name from a UNC path
// e.g., \\server\share -> share, //server/share -> share
func extractShareName(path string) string {
	// Remove leading slashes
	for len(path) > 0 && (path[0] == '\\' || path[0] == '/') {
		path = path[1:]
	}

	// Skip server name
	for i := 0; i < len(path); i++ {
		if path[i] == '\\' || path[i] == '/' {
			path = path[i+1:]
			break
		}
	}

	// Extract share name (up to next slash or end)
	for i := 0; i < len(path); i++ {
		if path[i] == '\\' || path[i] == '/' {
			return path[:i]
		}
	}
	return path
}

// now returns current time (can be mocked for testing)
var now = func() time.Time {
	return time.Now()
}

// Ensure handler methods are not unused
var _ = fmt.Sprint
