package smbfs

import (
	"log"
	"time"

	"github.com/absfs/absfs"
)

// ServerOptions defines server-level configuration
type ServerOptions struct {
	// Network settings
	Port     int    // Listen port (default: 445)
	Hostname string // Bind hostname (default: "0.0.0.0")

	// Protocol settings
	MinDialect      SMBDialect // Minimum SMB dialect to accept (default: SMB2_0_2)
	MaxDialect      SMBDialect // Maximum SMB dialect to offer (default: SMB3_1_1)
	SigningRequired bool       // Require message signing (default: false)

	// Connection settings
	MaxConnections int           // Maximum concurrent connections (0 = unlimited)
	IdleTimeout    time.Duration // Connection idle timeout (default: 15m)
	ReadTimeout    time.Duration // Read timeout per message (default: 30s)
	WriteTimeout   time.Duration // Write timeout per message (default: 30s)

	// Server identity
	ServerGUID [16]byte // Server GUID (generated if zero)
	ServerName string   // NetBIOS name (optional)

	// Authentication
	Users      map[string]string // Server-level users: username -> password
	AllowGuest bool              // Allow guest/anonymous access (default: true)

	// Logging
	Logger ServerLogger // Logger interface (optional)
	Debug  bool         // Enable debug logging

	// Performance
	MaxReadSize  uint32 // Maximum read size (default: 8MB)
	MaxWriteSize uint32 // Maximum write size (default: 8MB)
}

// DefaultServerOptions returns sensible default server options
func DefaultServerOptions() ServerOptions {
	return ServerOptions{
		Port:           445,
		Hostname:       "0.0.0.0",
		MinDialect:     SMB2_0_2,
		MaxDialect:     SMB3_1_1,
		MaxConnections: 100,
		IdleTimeout:    15 * time.Minute,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxReadSize:    MaxReadSize,
		MaxWriteSize:   MaxWriteSize,
		AllowGuest:     true, // Allow guest by default for easy testing
	}
}

// ShareOptions defines the configuration for an SMB share export
type ShareOptions struct {
	// Share identity
	ShareName string         // Share name (e.g., "data") - required
	SharePath string         // Root path within the filesystem (default: "/")
	ShareType SMBShareType   // Type of share (disk, pipe, etc.) - default: disk

	// Access control
	ReadOnly     bool              // Export as read-only
	AllowGuest   bool              // Allow anonymous/guest access
	AllowedUsers []string          // List of allowed usernames (nil = all authenticated users)
	AllowedIPs   []string          // List of allowed client IPs/subnets (nil = all)
	Users        map[string]string // username -> password for basic authentication

	// Share properties
	Comment      string // Share comment/description
	MaxUsers     int    // Maximum concurrent users (0 = unlimited)
	Hidden       bool   // Hide from share enumeration

	// Cache settings
	CachingMode CachingMode // Client-side caching mode
}

// SMBShareType represents the type of SMB share (different from ShareType in shares.go)
type SMBShareType uint8

const (
	SMBShareTypeDisk  SMBShareType = 0x01 // Standard disk share
	SMBShareTypePipe  SMBShareType = 0x02 // Named pipe share (IPC$)
	SMBShareTypePrint SMBShareType = 0x03 // Printer share
)

// CachingMode defines the client-side caching policy
type CachingMode uint32

const (
	CachingModeManual CachingMode = 0x00 // Manual caching (default)
	CachingModeAuto   CachingMode = 0x10 // Auto caching for documents
	CachingModeVDO    CachingMode = 0x20 // Auto caching for programs
	CachingModeNone   CachingMode = 0x30 // No caching
)

// DefaultShareOptions returns sensible default share options
func DefaultShareOptions(shareName string) ShareOptions {
	return ShareOptions{
		ShareName:   shareName,
		SharePath:   "/",
		AllowGuest:  true, // Start with guest access for simplicity
		CachingMode: CachingModeManual,
	}
}

// Share represents an SMB share backed by an absfs.FileSystem
type Share struct {
	fs          absfs.FileSystem
	options     ShareOptions
	fileHandles *FileHandleMap
}

// NewShare creates a new share
func NewShare(fs absfs.FileSystem, options ShareOptions) *Share {
	return &Share{
		fs:          fs,
		options:     options,
		fileHandles: NewFileHandleMap(),
	}
}

// FileSystem returns the underlying filesystem
func (s *Share) FileSystem() absfs.FileSystem {
	return s.fs
}

// Options returns the share options
func (s *Share) Options() ShareOptions {
	return s.options
}

// FileHandles returns the file handle map for this share
func (s *Share) FileHandles() *FileHandleMap {
	return s.fileHandles
}

// IsReadOnly returns true if the share is read-only
func (s *Share) IsReadOnly() bool {
	return s.options.ReadOnly
}

// GetShareType returns the SMB share type (disk, pipe, print)
func (s *Share) GetShareType() SMBShareType {
	if s.options.ShareType == 0 {
		return SMBShareTypeDisk // Default to disk
	}
	return s.options.ShareType
}

// AllowsGuest returns true if guest access is allowed
func (s *Share) AllowsGuest() bool {
	return s.options.AllowGuest
}

// CheckUserAccess verifies if a user is allowed to access this share
func (s *Share) CheckUserAccess(username string, isGuest bool) bool {
	// Guest check
	if isGuest {
		return s.options.AllowGuest
	}

	// If no user restrictions, allow all authenticated users
	if len(s.options.AllowedUsers) == 0 {
		return true
	}

	// Check if user is in allowed list
	for _, allowed := range s.options.AllowedUsers {
		if allowed == username {
			return true
		}
	}
	return false
}

// ValidateCredentials checks username/password against configured users
func (s *Share) ValidateCredentials(username, password string) bool {
	if len(s.options.Users) == 0 {
		// No users configured, rely on external authentication
		return true
	}

	storedPassword, ok := s.options.Users[username]
	if !ok {
		return false
	}
	return storedPassword == password
}

// ServerLogger defines the logging interface for the SMB server
type ServerLogger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// DefaultLogger wraps the standard log package
type DefaultLogger struct {
	debug bool
}

// NewDefaultLogger creates a default logger
func NewDefaultLogger(debug bool) *DefaultLogger {
	return &DefaultLogger{debug: debug}
}

func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	if l.debug {
		log.Printf("[DEBUG] "+msg, args...)
	}
}

func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	log.Printf("[INFO] "+msg, args...)
}

func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	log.Printf("[WARN] "+msg, args...)
}

func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	log.Printf("[ERROR] "+msg, args...)
}

// NullLogger discards all log messages
type NullLogger struct{}

func (l *NullLogger) Debug(msg string, args ...interface{}) {}
func (l *NullLogger) Info(msg string, args ...interface{})  {}
func (l *NullLogger) Warn(msg string, args ...interface{})  {}
func (l *NullLogger) Error(msg string, args ...interface{}) {}
