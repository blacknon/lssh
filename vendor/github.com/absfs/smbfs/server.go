package smbfs

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/absfs/absfs"
)

// Server represents an SMB server instance
type Server struct {
	options  ServerOptions
	shares   map[string]*Share
	sharesMu sync.RWMutex

	listener net.Listener
	handler  *SMBHandler
	sessions *SessionManager

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Connection tracking
	connMu     sync.Mutex
	conns      map[net.Conn]*connState
	connCount  int
	shutdownCh chan struct{}

	logger ServerLogger
}

// connState tracks state for each connection
type connState struct {
	conn            net.Conn
	session         *Session
	lastActive      time.Time
	remoteAddr      string
	dialect         SMBDialect // Negotiated dialect
	signingRequired bool       // Whether signing is required for this connection
	preauthHash     []byte     // SMB 3.1.1 preauth integrity hash (for key derivation)
}

// NewServer creates a new SMB server
func NewServer(options ServerOptions) (*Server, error) {
	// Apply defaults
	if options.Port == 0 {
		options.Port = 445
	}
	if options.Hostname == "" {
		options.Hostname = "0.0.0.0"
	}
	if options.MinDialect == 0 {
		options.MinDialect = SMB2_0_2
	}
	if options.MaxDialect == 0 {
		options.MaxDialect = SMB3_1_1
	}
	if options.IdleTimeout == 0 {
		options.IdleTimeout = 15 * time.Minute
	}
	if options.ReadTimeout == 0 {
		options.ReadTimeout = 30 * time.Second
	}
	if options.WriteTimeout == 0 {
		options.WriteTimeout = 30 * time.Second
	}
	if options.MaxReadSize == 0 {
		options.MaxReadSize = MaxReadSize
	}
	if options.MaxWriteSize == 0 {
		options.MaxWriteSize = MaxWriteSize
	}

	// Generate server GUID if not provided
	if options.ServerGUID == [16]byte{} {
		if _, err := rand.Read(options.ServerGUID[:]); err != nil {
			return nil, fmt.Errorf("failed to generate server GUID: %w", err)
		}
	}

	// Set up logger
	logger := options.Logger
	if logger == nil {
		logger = NewDefaultLogger(options.Debug)
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		options:    options,
		shares:     make(map[string]*Share),
		sessions:   NewSessionManager(options.IdleTimeout),
		ctx:        ctx,
		cancel:     cancel,
		conns:      make(map[net.Conn]*connState),
		shutdownCh: make(chan struct{}),
		logger:     logger,
	}

	s.handler = NewSMBHandler(s)

	// Automatically add IPC$ share (required by Windows)
	s.addIPCShare()

	return s, nil
}

// addIPCShare adds the special IPC$ share for Windows compatibility
func (s *Server) addIPCShare() {
	ipcShare := &Share{
		fs: nil, // IPC$ doesn't have a filesystem
		options: ShareOptions{
			ShareName:  "IPC$",
			ShareType:  SMBShareTypePipe,
			AllowGuest: true, // IPC$ typically allows guest
			Hidden:     true,
		},
		fileHandles: NewFileHandleMap(),
	}
	s.shares["IPC$"] = ipcShare
	s.logger.Debug("Added IPC$ share for Windows compatibility")
}

// AddShare registers a new share backed by an absfs.FileSystem
func (s *Server) AddShare(fs absfs.FileSystem, options ShareOptions) error {
	if options.ShareName == "" {
		return errors.New("share name is required")
	}

	// Normalize share name to uppercase (SMB convention)
	shareName := options.ShareName

	s.sharesMu.Lock()
	defer s.sharesMu.Unlock()

	if _, exists := s.shares[shareName]; exists {
		return fmt.Errorf("share %q already exists", shareName)
	}

	share := NewShare(fs, options)
	s.shares[shareName] = share

	s.logger.Info("Added share: %s (path: %s, readonly: %v, guest: %v)",
		shareName, options.SharePath, options.ReadOnly, options.AllowGuest)

	return nil
}

// RemoveShare removes a share
func (s *Server) RemoveShare(shareName string) error {
	s.sharesMu.Lock()
	defer s.sharesMu.Unlock()

	if _, exists := s.shares[shareName]; !exists {
		return fmt.Errorf("share %q not found", shareName)
	}

	delete(s.shares, shareName)
	s.logger.Info("Removed share: %s", shareName)
	return nil
}

// GetShare retrieves a share by name
func (s *Server) GetShare(shareName string) *Share {
	s.sharesMu.RLock()
	defer s.sharesMu.RUnlock()
	return s.shares[shareName]
}

// ListShares returns the names of all shares
func (s *Server) ListShares() []string {
	s.sharesMu.RLock()
	defer s.sharesMu.RUnlock()

	names := make([]string, 0, len(s.shares))
	for name, share := range s.shares {
		if !share.options.Hidden {
			names = append(names, name)
		}
	}
	return names
}

// Listen starts the server and begins accepting connections
func (s *Server) Listen() error {
	addr := fmt.Sprintf("%s:%d", s.options.Hostname, s.options.Port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.listener = listener
	s.logger.Info("SMB server listening on %s", addr)

	// Start session cleanup goroutine
	s.wg.Add(1)
	go s.sessionCleanupLoop()

	// Accept connections
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// ListenAndServe starts the server and blocks until shutdown
func (s *Server) ListenAndServe() error {
	if err := s.Listen(); err != nil {
		return err
	}
	<-s.ctx.Done()
	return nil
}

// Addr returns the server's listening address
func (s *Server) Addr() net.Addr {
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	s.logger.Info("Shutting down SMB server...")

	// Signal shutdown
	s.cancel()
	close(s.shutdownCh)

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	// Close all connections
	s.connMu.Lock()
	for conn := range s.conns {
		conn.Close()
	}
	s.connMu.Unlock()

	// Wait for goroutines to finish
	s.wg.Wait()

	s.logger.Info("SMB server stopped")
	return nil
}

// acceptLoop accepts new connections
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				s.logger.Error("Accept error: %v", err)
				continue
			}
		}

		// Check connection limit
		if s.options.MaxConnections > 0 {
			s.connMu.Lock()
			if s.connCount >= s.options.MaxConnections {
				s.connMu.Unlock()
				s.logger.Warn("Connection limit reached, rejecting connection from %s",
					conn.RemoteAddr())
				conn.Close()
				continue
			}
			s.connCount++
			s.connMu.Unlock()
		}

		// Handle connection
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection processes SMB messages from a connection
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer func() {
		conn.Close()
		s.connMu.Lock()
		delete(s.conns, conn)
		s.connCount--
		s.connMu.Unlock()
	}()

	remoteAddr := conn.RemoteAddr().String()
	s.logger.Debug("New connection from %s", remoteAddr)

	// Track connection
	state := &connState{
		conn:       conn,
		lastActive: time.Now(),
		remoteAddr: remoteAddr,
	}
	s.connMu.Lock()
	s.conns[conn] = state
	s.connMu.Unlock()

	// Message processing loop
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Set read deadline
		conn.SetReadDeadline(time.Now().Add(s.options.ReadTimeout))

		// Read message
		msg, err := s.readMessage(conn)
		if err != nil {
			if err == io.EOF || errors.Is(err, net.ErrClosed) {
				s.logger.Debug("Connection closed: %s", remoteAddr)
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				s.logger.Debug("Connection timeout: %s", remoteAddr)
			} else {
				s.logger.Error("Read error from %s: %v", remoteAddr, err)
			}
			return
		}

		// Update activity
		state.lastActive = time.Now()

		// For SMB 3.1.1, update preauth hash with request (NEGOTIATE or SESSION_SETUP before auth complete)
		if state.dialect >= SMB3_1_1 || msg.Header.Command == SMB2_NEGOTIATE {
			if msg.Header.Command == SMB2_NEGOTIATE ||
				(msg.Header.Command == SMB2_SESSION_SETUP && (state.session == nil || state.session.State != SessionStateValid)) {
				state.preauthHash = UpdatePreauthHash(state.preauthHash, msg.RawBytes)
			}
		}

		// Handle message
		response, err := s.handler.HandleMessage(state, msg)
		if err != nil {
			s.logger.Error("Handle error from %s: %v", remoteAddr, err)
			// Send error response if possible
			if response != nil {
				_, _ = s.writeMessage(conn, response)
			}
			continue
		}

		// Send response
		if response != nil {
			conn.SetWriteDeadline(time.Now().Add(s.options.WriteTimeout))
			responseBytes, err := s.writeMessage(conn, response)
			if err != nil {
				s.logger.Error("Write error to %s: %v", remoteAddr, err)
				return
			}

			// For SMB 3.1.1, update preauth hash with response (NEGOTIATE or SESSION_SETUP before auth complete)
			if state.dialect >= SMB3_1_1 {
				if response.Header.Command == SMB2_NEGOTIATE ||
					(response.Header.Command == SMB2_SESSION_SETUP && response.Header.Status != STATUS_SUCCESS) {
					state.preauthHash = UpdatePreauthHash(state.preauthHash, responseBytes)
				}
			}
		}
	}
}

// readMessage reads an SMB2 message from the connection
func (s *Server) readMessage(conn net.Conn) (*SMB2Message, error) {
	// Read NetBIOS header (4 bytes: 0x00 + 3-byte length)
	nbHeader := make([]byte, 4)
	if _, err := io.ReadFull(conn, nbHeader); err != nil {
		return nil, err
	}

	// Parse length (24-bit big-endian)
	msgLen := int(nbHeader[1])<<16 | int(nbHeader[2])<<8 | int(nbHeader[3])
	if msgLen < SMB2HeaderSize {
		return nil, ErrInvalidMessage
	}
	if msgLen > MaxTransactSize {
		return nil, ErrInvalidMessage
	}

	// Read SMB2 message
	msgData := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, msgData); err != nil {
		return nil, err
	}

	// Verify protocol signature
	if string(msgData[0:4]) != SMB2ProtocolID {
		// Check for SMB1 NEGOTIATE (0xFF 'S' 'M' 'B')
		if msgData[0] == 0xFF && string(msgData[1:4]) == "SMB" {
			return s.handleSMB1Negotiate(msgData)
		}
		return nil, ErrInvalidMessage
	}

	// Parse header
	header, err := UnmarshalSMB2Header(msgData)
	if err != nil {
		return nil, err
	}

	// Extract payload
	payload := msgData[SMB2HeaderSize:]

	return &SMB2Message{
		Header:   header,
		Payload:  payload,
		RawBytes: msgData, // Store raw bytes for preauth hash computation
	}, nil
}

// handleSMB1Negotiate handles SMB1 NEGOTIATE by returning SMB2 NEGOTIATE response
// This allows clients that start with SMB1 to negotiate up to SMB2
func (s *Server) handleSMB1Negotiate(data []byte) (*SMB2Message, error) {
	// Create a synthetic SMB2 NEGOTIATE request
	// The handler will respond with SMB2 NEGOTIATE response
	header := &SMB2Header{
		StructureSize: SMB2HeaderSize,
		Command:       SMB2_NEGOTIATE,
		MessageID:     0,
	}
	copy(header.ProtocolID[:], SMB2ProtocolID)

	return &SMB2Message{
		Header:  header,
		Payload: nil, // Empty payload signals SMB1 client upgrade
	}, nil
}

// writeMessage writes an SMB2 message to the connection
// Returns the raw SMB2 message bytes (without NetBIOS header) for preauth hash computation
func (s *Server) writeMessage(conn net.Conn, msg *SMB2Message) ([]byte, error) {
	// Marshal the message
	headerBytes := msg.Header.Marshal()
	msgLen := len(headerBytes) + len(msg.Payload)

	// Build NetBIOS header + SMB2 message
	buf := make([]byte, 4+msgLen)
	buf[0] = 0x00 // NetBIOS session message
	buf[1] = byte(msgLen >> 16)
	buf[2] = byte(msgLen >> 8)
	buf[3] = byte(msgLen)
	copy(buf[4:], headerBytes)
	copy(buf[4+SMB2HeaderSize:], msg.Payload)

	// Apply message signing if signing key is set
	if msg.SigningKey != nil && len(msg.SigningKey) > 0 {
		// Sign the SMB2 message (everything after NetBIOS header)
		smb2Message := buf[4:]
		signature := SignMessage(smb2Message, msg.SigningKey, msg.Dialect)
		if signature != nil {
			// Apply signature to the buffer
			ApplySignature(smb2Message, signature)
			s.logger.Debug("Applied message signature (dialect=%s)", msg.Dialect.String())
		}
	}

	_, err := conn.Write(buf)
	// Return SMB2 message bytes (without NetBIOS header) for preauth hash
	return buf[4:], err
}

// sessionCleanupLoop periodically cleans up expired sessions
func (s *Server) sessionCleanupLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			expired := s.sessions.CleanupExpired()
			for _, session := range expired {
				s.logger.Debug("Cleaned up expired session: %d", session.ID)
				// Clean up file handles for this session
				for _, share := range s.shares {
					share.fileHandles.ReleaseBySession(session.ID)
				}
			}
		}
	}
}

// Options returns the server options
func (s *Server) Options() ServerOptions {
	return s.options
}

// Sessions returns the session manager
func (s *Server) Sessions() *SessionManager {
	return s.sessions
}

// Logger returns the server logger
func (s *Server) Logger() ServerLogger {
	return s.logger
}

// ConnectionCount returns the current number of connections
func (s *Server) ConnectionCount() int {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	return s.connCount
}

// SessionCount returns the number of active sessions
func (s *Server) SessionCount() int {
	return s.sessions.SessionCount()
}

// Errors specific to the server
var (
	ErrInvalidMessage = errors.New("invalid SMB message")
	ErrServerClosed   = errors.New("server closed")
)

// generateMessageID generates a random message ID
func generateMessageID() uint64 {
	var buf [8]byte
	rand.Read(buf[:])
	return binary.LittleEndian.Uint64(buf[:])
}
