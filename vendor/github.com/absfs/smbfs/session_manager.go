package smbfs

import (
	"crypto/rand"
	"sync"
	"time"
)

// SessionState represents the state of an SMB session
type SessionState int

const (
	SessionStateInProgress SessionState = iota // SESSION_SETUP in progress
	SessionStateValid                          // Session is fully authenticated
	SessionStateExpired                        // Session has expired
)

// Session represents an authenticated SMB session
type Session struct {
	ID           uint64
	State        SessionState
	Dialect      SMBDialect
	IsGuest      bool
	Username     string
	Domain       string
	SigningKey   []byte
	CreatedAt    time.Time
	LastActivity time.Time

	// Connection info
	ClientGUID [16]byte
	ClientIP   string

	// Authentication state (for multi-step auth like NTLM)
	Authenticator Authenticator

	// Tree connections for this session
	mu     sync.RWMutex
	trees  map[uint32]*TreeConnection
	nextID uint32
}

// TreeConnection represents a connection to a share within a session
type TreeConnection struct {
	ID         uint32
	ShareName  string
	Share      *Share
	Session    *Session
	CreatedAt  time.Time
	IsReadOnly bool // Effective read-only status (share or session)
}

// SessionManager tracks active sessions
type SessionManager struct {
	mu        sync.RWMutex
	sessions  map[uint64]*Session
	byGUID    map[[16]byte]*Session // For reconnection support
	nextID    uint64
	idleLimit time.Duration
}

// NewSessionManager creates a new session manager
func NewSessionManager(idleLimit time.Duration) *SessionManager {
	if idleLimit == 0 {
		idleLimit = 15 * time.Minute // Default idle timeout
	}
	return &SessionManager{
		sessions:  make(map[uint64]*Session),
		byGUID:    make(map[[16]byte]*Session),
		nextID:    1,
		idleLimit: idleLimit,
	}
}

// CreateSession creates a new session in InProgress state
func (m *SessionManager) CreateSession(dialect SMBDialect, clientGUID [16]byte, clientIP string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate a random session ID
	var sessionID uint64
	var randomBytes [8]byte
	if _, err := rand.Read(randomBytes[:]); err == nil {
		sessionID = le.Uint64(randomBytes[:])
	}
	// Ensure non-zero
	if sessionID == 0 {
		sessionID = m.nextID
		m.nextID++
	}

	now := time.Now()
	session := &Session{
		ID:           sessionID,
		State:        SessionStateInProgress,
		Dialect:      dialect,
		ClientGUID:   clientGUID,
		ClientIP:     clientIP,
		CreatedAt:    now,
		LastActivity: now,
		trees:        make(map[uint32]*TreeConnection),
		nextID:       1,
	}

	m.sessions[sessionID] = session
	if clientGUID != [16]byte{} {
		m.byGUID[clientGUID] = session
	}

	return session
}

// GetSession retrieves a session by ID
func (m *SessionManager) GetSession(id uint64) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// GetSessionByGUID retrieves a session by client GUID (for reconnection)
func (m *SessionManager) GetSessionByGUID(guid [16]byte) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byGUID[guid]
}

// ValidateSession checks if a session exists and is valid
func (m *SessionManager) ValidateSession(id uint64) (*Session, NTStatus) {
	m.mu.RLock()
	session := m.sessions[id]
	m.mu.RUnlock()

	if session == nil {
		return nil, STATUS_USER_SESSION_DELETED
	}
	if session.State != SessionStateValid {
		return nil, STATUS_USER_SESSION_DELETED
	}
	return session, STATUS_SUCCESS
}

// UpdateActivity updates the last activity time for a session
func (m *SessionManager) UpdateActivity(id uint64) {
	m.mu.RLock()
	session := m.sessions[id]
	m.mu.RUnlock()

	if session != nil {
		session.mu.Lock()
		session.LastActivity = time.Now()
		session.mu.Unlock()
	}
}

// DestroySession removes a session and all its tree connections
func (m *SessionManager) DestroySession(id uint64) *Session {
	m.mu.Lock()
	session := m.sessions[id]
	if session != nil {
		delete(m.sessions, id)
		if session.ClientGUID != [16]byte{} {
			delete(m.byGUID, session.ClientGUID)
		}
	}
	m.mu.Unlock()
	return session
}

// CleanupExpired removes expired/idle sessions
func (m *SessionManager) CleanupExpired() []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	var expired []*Session
	cutoff := time.Now().Add(-m.idleLimit)

	for id, session := range m.sessions {
		session.mu.RLock()
		lastActivity := session.LastActivity
		session.mu.RUnlock()

		if lastActivity.Before(cutoff) {
			expired = append(expired, session)
			delete(m.sessions, id)
			if session.ClientGUID != [16]byte{} {
				delete(m.byGUID, session.ClientGUID)
			}
		}
	}
	return expired
}

// SessionCount returns the number of active sessions
func (m *SessionManager) SessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// --- Session methods ---

// SetValid marks the session as fully authenticated
func (s *Session) SetValid(username, domain string, isGuest bool, signingKey []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.State = SessionStateValid
	s.Username = username
	s.Domain = domain
	s.IsGuest = isGuest
	s.SigningKey = signingKey
	s.LastActivity = time.Now()
}

// AddTreeConnection adds a tree connection to the session
func (s *Session) AddTreeConnection(shareName string, share *Share, readOnly bool) *TreeConnection {
	s.mu.Lock()
	defer s.mu.Unlock()

	treeID := s.nextID
	s.nextID++

	tree := &TreeConnection{
		ID:         treeID,
		ShareName:  shareName,
		Share:      share,
		Session:    s,
		CreatedAt:  time.Now(),
		IsReadOnly: readOnly,
	}

	s.trees[treeID] = tree
	return tree
}

// GetTreeConnection retrieves a tree connection by ID
func (s *Session) GetTreeConnection(id uint32) *TreeConnection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.trees[id]
}

// RemoveTreeConnection removes a tree connection from the session
func (s *Session) RemoveTreeConnection(id uint32) *TreeConnection {
	s.mu.Lock()
	defer s.mu.Unlock()

	tree := s.trees[id]
	delete(s.trees, id)
	return tree
}

// TreeCount returns the number of tree connections in this session
func (s *Session) TreeCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.trees)
}

// GetAllTreeConnections returns all tree connections (for cleanup)
func (s *Session) GetAllTreeConnections() []*TreeConnection {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trees := make([]*TreeConnection, 0, len(s.trees))
	for _, tree := range s.trees {
		trees = append(trees, tree)
	}
	return trees
}

// ValidateTreeConnection checks if a tree connection exists and is valid
func (s *Session) ValidateTreeConnection(id uint32) (*TreeConnection, NTStatus) {
	s.mu.RLock()
	tree := s.trees[id]
	s.mu.RUnlock()

	if tree == nil {
		return nil, STATUS_NETWORK_NAME_DELETED
	}
	return tree, STATUS_SUCCESS
}
