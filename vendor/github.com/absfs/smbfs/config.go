package smbfs

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Logger interface for logging operations.
type Logger interface {
	Printf(format string, v ...interface{})
}

// Config holds the configuration for an SMB filesystem connection.
type Config struct {
	// Server connection
	Server string // Hostname or IP address
	Port   int    // SMB port (default: 445)
	Share  string // Share name

	// Authentication
	Username    string // Username (domain\user or user@domain)
	Password    string // Password
	Domain      string // Domain name (optional)
	UseKerberos bool   // Use Kerberos authentication
	GuestAccess bool   // Anonymous/guest access

	// SMB protocol
	Dialect    string // Preferred dialect (SMB2, SMB3, etc.)
	Signing    bool   // Require message signing
	Encryption bool   // Require encryption (SMB3+)

	// Connection pool
	MaxIdle     int           // Max idle connections (default: 5)
	MaxOpen     int           // Max open connections (default: 10)
	IdleTimeout time.Duration // Idle timeout (default: 5m)
	ConnTimeout time.Duration // Connection timeout (default: 30s)
	OpTimeout   time.Duration // Operation timeout (default: 60s)

	// Behavior
	CaseSensitive  bool // Case-sensitive paths (default: false)
	FollowSymlinks bool // Follow Windows symlinks/junctions

	// Performance
	ReadBufferSize  int         // Read buffer size (default: 64KB)
	WriteBufferSize int         // Write buffer size (default: 64KB)
	Cache           CacheConfig // Metadata caching configuration

	// Retry and reliability
	RetryPolicy *RetryPolicy // Retry policy for failed operations (nil = use default)

	// Logging
	Logger Logger // Logger for debug and error messages (nil = no logging)
}

// setDefaults sets default values for any unspecified configuration options.
func (c *Config) setDefaults() {
	if c.Port == 0 {
		c.Port = 445
	}
	if c.MaxIdle == 0 {
		c.MaxIdle = 5
	}
	if c.MaxOpen == 0 {
		c.MaxOpen = 10
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = 5 * time.Minute
	}
	if c.ConnTimeout == 0 {
		c.ConnTimeout = 30 * time.Second
	}
	if c.OpTimeout == 0 {
		c.OpTimeout = 60 * time.Second
	}
	if c.ReadBufferSize == 0 {
		c.ReadBufferSize = 64 * 1024 // 64KB
	}
	if c.WriteBufferSize == 0 {
		c.WriteBufferSize = 64 * 1024 // 64KB
	}
	// Set default cache config if not specified
	if c.Cache.MaxCacheEntries == 0 {
		c.Cache = DefaultCacheConfig()
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.Server == "" {
		return fmt.Errorf("server is required")
	}
	if c.Share == "" {
		return fmt.Errorf("share is required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}

	// Validate authentication
	if !c.GuestAccess {
		if c.Username == "" {
			return fmt.Errorf("username is required for non-guest access")
		}
		if !c.UseKerberos && c.Password == "" {
			return fmt.Errorf("password is required when not using Kerberos")
		}
	}

	return nil
}

// ParseConnectionString parses an SMB connection string into a Config.
// Supported formats:
//   smb://[domain\]username:password@server[:port]/share[/path]
//   smb://server/share  // Guest access
//   smb://user:pass@server/share
//   smb://DOMAIN\user:pass@server/share
//   smb://server:10445/share  // Non-standard port
func ParseConnectionString(connStr string) (*Config, error) {
	u, err := url.Parse(connStr)
	if err != nil {
		return nil, fmt.Errorf("invalid connection string: %w", err)
	}

	if u.Scheme != "smb" {
		return nil, fmt.Errorf("invalid scheme: %s (expected 'smb')", u.Scheme)
	}

	cfg := &Config{
		Server: u.Hostname(),
		Port:   445, // default
	}

	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
		cfg.Port = port
	}

	// Extract share from path
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) > 0 && parts[0] != "" {
		cfg.Share = parts[0]
	}

	// Extract credentials
	if u.User != nil {
		username := u.User.Username()
		if password, ok := u.User.Password(); ok {
			cfg.Password = password
		}

		// Handle domain\user format
		if strings.Contains(username, "\\") {
			domainUser := strings.SplitN(username, "\\", 2)
			if len(domainUser) == 2 {
				cfg.Domain = domainUser[0]
				cfg.Username = domainUser[1]
			}
		} else {
			cfg.Username = username
		}
	} else {
		// No credentials means guest access
		cfg.GuestAccess = true
	}

	cfg.setDefaults()

	return cfg, nil
}
