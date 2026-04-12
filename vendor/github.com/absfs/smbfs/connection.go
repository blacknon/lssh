package smbfs

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/hirochachacha/go-smb2"
)

// connectionPool manages a pool of SMB connections.
type connectionPool struct {
	config  *Config
	factory ConnectionFactory

	mu          sync.Mutex
	connections []*pooledConn
	waiters     []chan *pooledConn
	numOpen     int
	closed      bool
}

// pooledConn wraps an SMB connection with metadata.
type pooledConn struct {
	session   SMBSession
	share     SMBShare
	createdAt time.Time
	lastUsed  time.Time
	inUse     bool
	mu        sync.Mutex
}

// newConnectionPool creates a new connection pool.
func newConnectionPool(config *Config) *connectionPool {
	return &connectionPool{
		config:      config,
		factory:     nil, // Uses default createConnection
		connections: make([]*pooledConn, 0, config.MaxOpen),
		waiters:     make([]chan *pooledConn, 0),
	}
}

// newConnectionPoolWithFactory creates a new connection pool with a custom factory.
// This is used for testing with mock connections.
func newConnectionPoolWithFactory(config *Config, factory ConnectionFactory) *connectionPool {
	return &connectionPool{
		config:      config,
		factory:     factory,
		connections: make([]*pooledConn, 0, config.MaxOpen),
		waiters:     make([]chan *pooledConn, 0),
	}
}

// get acquires a connection from the pool.
func (p *connectionPool) get(ctx context.Context) (*pooledConn, error) {
	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()
		return nil, ErrConnectionClosed
	}

	// Check for idle connections
	for i, conn := range p.connections {
		if !conn.inUse {
			// Check if connection is still valid and not expired
			if time.Since(conn.lastUsed) < p.config.IdleTimeout {
				conn.inUse = true
				conn.lastUsed = time.Now()
				p.mu.Unlock()
				return conn, nil
			}

			// Connection expired, close and remove it
			p.connections = append(p.connections[:i], p.connections[i+1:]...)
			p.numOpen--
			go conn.close()
		}
	}

	// Can we create a new connection?
	if p.numOpen < p.config.MaxOpen {
		p.numOpen++
		p.mu.Unlock()

		conn, err := p.createConnection(ctx)
		if err != nil {
			p.mu.Lock()
			p.numOpen--
			p.mu.Unlock()
			return nil, err
		}

		return conn, nil
	}

	// Wait for a connection to become available
	waiter := make(chan *pooledConn, 1)
	p.waiters = append(p.waiters, waiter)
	p.mu.Unlock()

	select {
	case conn := <-waiter:
		if conn == nil {
			return nil, ErrPoolExhausted
		}
		return conn, nil
	case <-ctx.Done():
		// Remove ourselves from waiters
		p.mu.Lock()
		for i, w := range p.waiters {
			if w == waiter {
				p.waiters = append(p.waiters[:i], p.waiters[i+1:]...)
				break
			}
		}
		p.mu.Unlock()
		return nil, ctx.Err()
	case <-time.After(p.config.ConnTimeout):
		return nil, ErrPoolExhausted
	}
}

// put returns a connection to the pool.
func (p *connectionPool) put(conn *pooledConn) {
	if conn == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		go conn.close()
		return
	}

	conn.inUse = false
	conn.lastUsed = time.Now()

	// Try to give the connection to a waiter
	if len(p.waiters) > 0 {
		waiter := p.waiters[0]
		p.waiters = p.waiters[1:]
		conn.inUse = true
		waiter <- conn
		return
	}

	// Keep connection in the pool if under MaxIdle
	idleCount := 0
	for _, c := range p.connections {
		if !c.inUse {
			idleCount++
		}
	}

	if idleCount >= p.config.MaxIdle {
		// Too many idle connections, close this one
		p.numOpen--
		for i, c := range p.connections {
			if c == conn {
				p.connections = append(p.connections[:i], p.connections[i+1:]...)
				break
			}
		}
		go conn.close()
	}
}

// createConnection creates a new SMB connection.
func (p *connectionPool) createConnection(ctx context.Context) (*pooledConn, error) {
	// Use factory if available (for testing)
	if p.factory != nil {
		session, share, err := p.factory.CreateConnection(p.config)
		if err != nil {
			return nil, err
		}

		conn := &pooledConn{
			session:   session,
			share:     share,
			createdAt: time.Now(),
			lastUsed:  time.Now(),
			inUse:     true,
		}

		p.mu.Lock()
		p.connections = append(p.connections, conn)
		p.mu.Unlock()

		return conn, nil
	}

	// Default behavior: create real SMB connection
	return p.createRealConnection(ctx)
}

// createRealConnection creates a real SMB connection using go-smb2.
func (p *connectionPool) createRealConnection(ctx context.Context) (*pooledConn, error) {
	addr := fmt.Sprintf("%s:%d", p.config.Server, p.config.Port)

	if p.config.Logger != nil {
		p.config.Logger.Printf("Creating new SMB connection to %s", addr)
	}

	// Create TCP connection with timeout
	dialer := &net.Dialer{
		Timeout: p.config.ConnTimeout,
	}

	netConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		if p.config.Logger != nil {
			p.config.Logger.Printf("Failed to connect to %s: %v", addr, err)
		}
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	// Create SMB session
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     p.config.Username,
			Password: p.config.Password,
			Domain:   p.config.Domain,
		},
	}

	session, err := d.Dial(netConn)
	if err != nil {
		netConn.Close()
		if p.config.Logger != nil {
			p.config.Logger.Printf("SMB session setup failed: %v", err)
		}
		return nil, fmt.Errorf("SMB session setup failed: %w", err)
	}

	// Connect to share
	share, err := session.Mount(p.config.Share)
	if err != nil {
		_ = session.Logoff()
		netConn.Close()
		if p.config.Logger != nil {
			p.config.Logger.Printf("Failed to mount share %s: %v", p.config.Share, err)
		}
		return nil, fmt.Errorf("failed to mount share %s: %w", p.config.Share, err)
	}

	conn := &pooledConn{
		session:   &realSMBSession{session: session},
		share:     &realSMBShare{share: share},
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		inUse:     true,
	}

	p.mu.Lock()
	p.connections = append(p.connections, conn)
	p.mu.Unlock()

	if p.config.Logger != nil {
		p.config.Logger.Printf("Successfully created SMB connection to %s (total connections: %d)", addr, p.numOpen)
	}

	return conn, nil
}

// close closes a pooled connection.
func (pc *pooledConn) close() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.share != nil {
		_ = pc.share.Umount()
		pc.share = nil
	}

	if pc.session != nil {
		_ = pc.session.Logoff()
		pc.session = nil
	}
}

// Close closes all connections in the pool.
func (p *connectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	if p.config.Logger != nil {
		p.config.Logger.Printf("Closing connection pool (%d connections)", len(p.connections))
	}

	p.closed = true

	// Notify all waiters
	for _, waiter := range p.waiters {
		close(waiter)
	}
	p.waiters = nil

	// Close all connections
	for _, conn := range p.connections {
		go conn.close()
	}

	p.connections = nil
	p.numOpen = 0

	return nil
}

// cleanup removes expired idle connections.
func (p *connectionPool) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	now := time.Now()
	i := 0
	for _, conn := range p.connections {
		if !conn.inUse && now.Sub(conn.lastUsed) > p.config.IdleTimeout {
			// Connection expired
			p.numOpen--
			go conn.close()
			continue
		}
		p.connections[i] = conn
		i++
	}
	p.connections = p.connections[:i]
}

// startCleanup starts a background goroutine to clean up expired connections.
func (p *connectionPool) startCleanup(ctx context.Context) {
	ticker := time.NewTicker(p.config.IdleTimeout / 2)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.cleanup()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stats returns pool statistics for monitoring.
type PoolStats struct {
	TotalConnections int
	ActiveConnections int
	IdleConnections  int
	WaitersCount     int
	IsClosed         bool
}

// Stats returns current pool statistics.
func (p *connectionPool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	active := 0
	idle := 0
	for _, conn := range p.connections {
		if conn.inUse {
			active++
		} else {
			idle++
		}
	}

	return PoolStats{
		TotalConnections:  len(p.connections),
		ActiveConnections: active,
		IdleConnections:   idle,
		WaitersCount:      len(p.waiters),
		IsClosed:          p.closed,
	}
}
