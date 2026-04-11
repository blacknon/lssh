# smbfs

[![Go Reference](https://pkg.go.dev/badge/github.com/absfs/smbfs.svg)](https://pkg.go.dev/github.com/absfs/smbfs)
[![Go Report Card](https://goreportcard.com/badge/github.com/absfs/smbfs)](https://goreportcard.com/report/github.com/absfs/smbfs)
[![CI](https://github.com/absfs/smbfs/actions/workflows/ci.yml/badge.svg)](https://github.com/absfs/smbfs/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

SMB/CIFS network filesystem implementation for absfs - access Windows file shares and Samba servers through the absfs.FileSystem interface.

**Project Status:** ✅ Production Ready | **Phase:** 4 of 5 (80% Complete) | **Stability:** Stable

📊 [Project Status](PROJECT_STATUS.md) | 🚀 [Deployment Guide](DEPLOYMENT.md) | ⚡ [Performance Guide](PERFORMANCE.md) | 🔧 [Troubleshooting](TROUBLESHOOTING.md) | 🔐 [Security Audit](SECURITY.md)

## Overview

`smbfs` provides a complete absfs.FileSystem implementation for SMB/CIFS network shares, enabling seamless access to Windows shares, Samba servers, and other SMB-compatible network storage. It supports modern SMB2/SMB3 protocols with enterprise-grade authentication methods including NTLM, Kerberos, and domain authentication.

**Key Features:**
- **Full absfs.FileSystem interface compliance**
- **SMB2/SMB3/SMB3.1.1 protocol support** with automatic dialect negotiation
- **Multiple authentication methods**: NTLM, Kerberos, domain, guest access
- **Performance optimizations**: Metadata caching, connection pooling, configurable buffers
- **Production ready**: Retry logic, timeout handling, comprehensive logging
- **Windows attributes support**: Hidden, system, readonly, archive flags
- **Share enumeration**: List available shares on SMB servers
- **Security approved**: ✅ OWASP Top 10 compliant, no critical vulnerabilities
- **Cross-platform**: Windows, Linux, macOS
- **Large file support**: Files >4GB fully supported
- **Composable**: Works with other absfs implementations (cachefs, metricsfs, etc.)

## SMB Protocol Support

### Supported Dialects
- **SMB 2.0.2** - Basic SMB2 support
- **SMB 2.1** - Windows 7 / Server 2008 R2
- **SMB 3.0** - Windows 8 / Server 2012 (encryption, multichannel)
- **SMB 3.0.2** - Windows 8.1 / Server 2012 R2
- **SMB 3.1.1** - Windows 10 / Server 2016+ (enhanced security, integrity)

### Dialect Negotiation
The implementation automatically negotiates the highest supported dialect with the server:
1. Client sends list of supported dialects
2. Server responds with selected dialect
3. Connection established using negotiated dialect
4. Fallback to lower dialects if needed

## Architecture Design

```
┌─────────────────────────────────────────┐
│         Application Code                │
└────────────────┬────────────────────────┘
                 │
                 │ absfs.FileSystem interface
                 │
┌────────────────▼────────────────────────┐
│           smbfs.FileSystem              │
│  ┌─────────────────────────────────┐   │
│  │   Connection Pool Manager       │   │
│  │   - Session lifecycle           │   │
│  │   - Connection reuse            │   │
│  │   - Timeout handling            │   │
│  └─────────────────────────────────┘   │
│  ┌─────────────────────────────────┐   │
│  │   Authentication Manager        │   │
│  │   - NTLM authentication         │   │
│  │   - Kerberos integration        │   │
│  │   - Domain credentials          │   │
│  └─────────────────────────────────┘   │
│  ┌─────────────────────────────────┐   │
│  │   File Operations Mapper        │   │
│  │   - Path translation            │   │
│  │   - Attribute mapping           │   │
│  │   - Permission handling         │   │
│  └─────────────────────────────────┘   │
└────────────────┬────────────────────────┘
                 │
                 │ SMB2/SMB3 protocol
                 │
┌────────────────▼────────────────────────┐
│    github.com/hirochachacha/go-smb2    │
│  - Protocol implementation              │
│  - Message encoding/decoding            │
│  - Transport layer (TCP)                │
└────────────────┬────────────────────────┘
                 │
                 │ TCP/IP
                 │
┌────────────────▼────────────────────────┐
│         SMB Server                      │
│  - Windows File Sharing                 │
│  - Samba Server                         │
│  - NAS Devices                          │
└─────────────────────────────────────────┘
```

## Library Integration

Primary library: [github.com/hirochachacha/go-smb2](https://github.com/hirochachacha/go-smb2)

**Why go-smb2:**
- Pure Go implementation (no CGO dependencies)
- SMB2/SMB3 protocol support
- Active maintenance and updates
- Cross-platform compatibility
- Clean API design
- Good performance characteristics

**Alternative considered:**
- `github.com/stacktitan/smb` - Older, less maintained
- CGO bindings to libsmbclient - Platform dependency issues

## Authentication Methods

### Username/Password (NTLM)
Standard NTLM authentication with username and password:

```go
fs, err := smbfs.New(&smbfs.Config{
    Server:   "fileserver.example.com",
    Share:    "shared",
    Username: "jdoe",
    Password: "secret123",
    Domain:   "",  // Optional, for standalone servers
})
```

### Kerberos Authentication
Enterprise Kerberos authentication using system credentials:

```go
fs, err := smbfs.New(&smbfs.Config{
    Server:      "fileserver.corp.example.com",
    Share:       "departments",
    UseKerberos: true,
    Domain:      "CORP",
    Username:    "jdoe",
    // Kerberos ticket used instead of password
})
```

**Requirements:**
- Properly configured `/etc/krb5.conf` (Linux/macOS)
- Valid Kerberos ticket (via `kinit`)
- DNS resolution for domain controllers
- System time synchronization (critical for Kerberos)

### Guest Access
Anonymous/guest access for public shares:

```go
fs, err := smbfs.New(&smbfs.Config{
    Server:      "public.example.com",
    Share:       "public",
    GuestAccess: true,
})
```

### Domain Authentication
Active Directory domain authentication:

```go
fs, err := smbfs.New(&smbfs.Config{
    Server:   "fileserver.corp.example.com",
    Share:    "departments",
    Username: "jdoe",
    Password: "secret123",
    Domain:   "CORP",  // AD domain
})
```

## Implementation Details

### Connection Pooling

Connection pool manages SMB sessions efficiently:

```go
type ConnectionPool struct {
    // Pool configuration
    MaxIdle       int           // Maximum idle connections
    MaxOpen       int           // Maximum open connections
    IdleTimeout   time.Duration // Idle connection timeout
    ConnTimeout   time.Duration // Connection establishment timeout

    // Pool state
    mu          sync.Mutex
    connections []*pooledConn
    waiters     []chan *pooledConn
}

type pooledConn struct {
    conn      *smb2.Session
    share     *smb2.Share
    createdAt time.Time
    lastUsed  time.Time
    inUse     bool
}
```

**Pool behavior:**
- Lazy connection establishment
- Connection reuse across operations
- Automatic cleanup of idle connections
- Graceful handling of server disconnections
- Thread-safe connection checkout/checkin

### Session Management

SMB session lifecycle management:

1. **Connection Establishment**
   - TCP connection to server:445
   - SMB dialect negotiation
   - Session setup (authentication)
   - Tree connect to share

2. **Session Maintenance**
   - Keep-alive messages
   - Reconnection on timeout
   - Credential refresh (Kerberos)
   - Error recovery

3. **Session Cleanup**
   - Tree disconnect
   - Session logoff
   - TCP connection close
   - Resource cleanup

### Share Enumeration

List available shares on a server:

```go
type ShareInfo struct {
    Name    string      // Share name
    Type    ShareType   // Disk, Print, IPC, etc.
    Comment string      // Share description
}

func (fs *FileSystem) ListShares(ctx context.Context) ([]ShareInfo, error)
```

**Share types:**
- `STYPE_DISKTREE` - Disk share (standard file share)
- `STYPE_PRINTQ` - Print queue
- `STYPE_DEVICE` - Communication device
- `STYPE_IPC` - IPC share (named pipes)
- `STYPE_TEMPORARY` - Temporary share
- `STYPE_SPECIAL` - Special share (admin shares: C$, IPC$, etc.)

### File Operations Mapping

Mapping absfs operations to SMB protocol:

| absfs Operation | SMB2 Command | Notes |
|----------------|--------------|-------|
| `Open()` | CREATE | With desired access and disposition |
| `OpenFile()` | CREATE | With specific flags mapping |
| `Stat()` | QUERY_INFO | File standard/basic info |
| `ReadDir()` | QUERY_DIRECTORY | With pattern matching |
| `Remove()` | SET_INFO + DELETE | Mark for deletion on close |
| `Rename()` | SET_INFO | Set file name info |
| `Mkdir()` | CREATE | With directory attribute |
| `Chmod()` | SET_INFO | Security descriptor |
| `Chtimes()` | SET_INFO | File basic info |

### Directory Operations

Efficient directory traversal:

```go
// ReadDir implementation with pagination
func (fs *FileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
    file, err := fs.openDir(name)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    // Use SMB2 QUERY_DIRECTORY with resume
    var entries []fs.DirEntry
    resumeKey := uint32(0)

    for {
        batch, newKey, err := fs.queryDirectory(file, resumeKey)
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, err
        }

        entries = append(entries, batch...)
        resumeKey = newKey
    }

    return entries, nil
}
```

**Optimizations:**
- Batch directory queries (up to 64KB per request)
- Resume token handling for large directories
- Caching of directory metadata
- Parallel stat operations when needed

### Permission Handling (Windows ACLs)

Windows ACL to Unix permission mapping:

```go
type ACLMapper struct {
    // Map Windows SIDs to Unix UIDs/GIDs
    sidToUID map[string]uint32
    sidToGID map[string]uint32

    // Default mappings
    defaultUID uint32
    defaultGID uint32
    defaultMode os.FileMode
}

// Convert Windows security descriptor to Unix mode
func (m *ACLMapper) ACLToMode(sd *SecurityDescriptor) os.FileMode {
    mode := os.FileMode(0)

    // Owner permissions from DACL
    if sd.hasAccess(sd.Owner, FILE_READ_DATA) {
        mode |= 0400
    }
    if sd.hasAccess(sd.Owner, FILE_WRITE_DATA) {
        mode |= 0200
    }
    if sd.hasAccess(sd.Owner, FILE_EXECUTE) {
        mode |= 0100
    }

    // Group and other permissions...

    return mode
}

// Convert Unix mode to Windows security descriptor
func (m *ACLMapper) ModeToACL(mode os.FileMode) *SecurityDescriptor
```

**Permission mapping:**
- Owner → Primary owner SID
- Group → Primary group SID
- Other → Everyone SID
- Read → FILE_READ_DATA | FILE_READ_EA | FILE_READ_ATTRIBUTES
- Write → FILE_WRITE_DATA | FILE_WRITE_EA | FILE_WRITE_ATTRIBUTES
- Execute → FILE_EXECUTE

## Configuration Options

```go
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
    Dialect     string        // Preferred dialect (SMB2, SMB3, etc.)
    Signing     bool          // Require message signing
    Encryption  bool          // Require encryption (SMB3+)

    // Connection pool
    MaxIdle     int           // Max idle connections (default: 5)
    MaxOpen     int           // Max open connections (default: 10)
    IdleTimeout time.Duration // Idle timeout (default: 5m)
    ConnTimeout time.Duration // Connection timeout (default: 30s)
    OpTimeout   time.Duration // Operation timeout (default: 60s)

    // Behavior
    CaseSensitive bool        // Case-sensitive paths (default: false)
    FollowSymlinks bool       // Follow Windows symlinks/junctions

    // Performance
    ReadBufferSize  int       // Read buffer size (default: 64KB)
    WriteBufferSize int       // Write buffer size (default: 64KB)
    DirectoryCache  bool      // Enable directory metadata caching
    CacheTTL        time.Duration // Cache TTL (default: 30s)
}
```

### Connection String Format

Alternative connection string syntax:

```
smb://[domain\]username:password@server[:port]/share[/path]
smb://server/share  // Guest access
smb://user:pass@server/share
smb://DOMAIN\user:pass@server/share
smb://server:10445/share  // Non-standard port
```

Parsing:

```go
func ParseConnectionString(connStr string) (*Config, error) {
    u, err := url.Parse(connStr)
    if err != nil {
        return nil, err
    }

    cfg := &Config{
        Server: u.Hostname(),
        Port:   445, // default
    }

    if u.Port() != "" {
        cfg.Port, _ = strconv.Atoi(u.Port())
    }

    // Extract share from path
    parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
    if len(parts) > 0 {
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
            parts := strings.SplitN(username, "\\", 2)
            cfg.Domain = parts[0]
            cfg.Username = parts[1]
        } else {
            cfg.Username = username
        }
    }

    return cfg, nil
}
```

## Technical Specifications

### SMB Dialect Negotiation

```go
type DialectNegotiator struct {
    preferredDialects []uint16
    requiredFeatures  uint32
}

// Dialect constants
const (
    DialectSmb202  = 0x0202  // SMB 2.0.2
    DialectSmb21   = 0x0210  // SMB 2.1
    DialectSmb30   = 0x0300  // SMB 3.0
    DialectSmb302  = 0x0302  // SMB 3.0.2
    DialectSmb311  = 0x0311  // SMB 3.1.1
)

// Feature flags
const (
    FeatureDFS         = 0x00000001
    FeatureLeasing     = 0x00000002
    FeatureLargeMTU    = 0x00000004
    FeatureMultiChannel = 0x00000008
    FeaturePersistent  = 0x00000010
    FeatureDirectory   = 0x00000020
    FeatureEncryption  = 0x00000040
)

func (dn *DialectNegotiator) Negotiate(ctx context.Context, conn net.Conn) (uint16, error) {
    // Send negotiate request with supported dialects
    req := &NegotiateRequest{
        Dialects: dn.preferredDialects,
        SecurityMode: NEGOTIATE_SIGNING_ENABLED,
        Capabilities: dn.requiredFeatures,
    }

    // Receive negotiate response
    resp, err := sendNegotiate(ctx, conn, req)
    if err != nil {
        return 0, err
    }

    // Validate selected dialect
    selectedDialect := resp.DialectRevision
    if !contains(dn.preferredDialects, selectedDialect) {
        return 0, ErrUnsupportedDialect
    }

    return selectedDialect, nil
}
```

### Credential Management

#### Environment Variables

```bash
# Basic authentication
export SMB_USERNAME="jdoe"
export SMB_PASSWORD="secret123"
export SMB_DOMAIN="CORP"

# Kerberos
export SMB_USE_KERBEROS="true"
export KRB5_CONFIG="/etc/krb5.conf"
export KRB5CCNAME="/tmp/krb5cc_1000"

# Server configuration
export SMB_SERVER="fileserver.example.com"
export SMB_SHARE="shared"
```

#### Configuration File

`~/.config/smbfs/config.yaml`:

```yaml
servers:
  - name: corporate
    server: fileserver.corp.example.com
    domain: CORP
    username: jdoe
    # Password in keyring or separate credentials file
    use_kerberos: true
    shares:
      - name: departments
        path: /
      - name: home
        path: /home/jdoe

  - name: nas
    server: nas.home.local
    username: admin
    # Password in keyring
    shares:
      - name: media
        path: /media
      - name: backups
        path: /backups

# Global settings
connection_pool:
  max_idle: 5
  max_open: 10
  idle_timeout: 5m

security:
  require_signing: true
  require_encryption: false
  min_dialect: SMB3
```

Credentials file `~/.config/smbfs/credentials`:

```ini
[corporate]
username=jdoe
password=secret123

[nas]
username=admin
password=nasadmin123
```

**Security:**
- Credentials file should be mode 0600
- Support system keyring integration (keychain, gnome-keyring, etc.)
- Never log passwords
- Clear password from memory after use

### Share Path Handling

Normalize different path formats:

```go
type PathNormalizer struct {
    separator rune
    caseSensitive bool
}

// Support multiple formats:
// Windows: \\server\share\path\to\file
// Unix-style: /server/share/path/to/file
// SMB URL: smb://server/share/path/to/file
func (pn *PathNormalizer) Normalize(path string) string {
    // Convert Windows separators
    path = strings.ReplaceAll(path, "\\", "/")

    // Remove leading // or \\
    path = strings.TrimPrefix(path, "//")

    // Remove server and share components (already in connection)
    parts := strings.Split(path, "/")
    if len(parts) >= 2 {
        // Skip server and share parts
        path = "/" + strings.Join(parts[2:], "/")
    }

    // Clean path
    path = filepath.Clean(path)

    // Case normalization (Windows is case-insensitive)
    if !pn.caseSensitive {
        path = strings.ToLower(path)
    }

    return path
}
```

### Windows-Specific File Attributes

```go
// Windows file attribute flags
const (
    FILE_ATTRIBUTE_READONLY            = 0x00000001
    FILE_ATTRIBUTE_HIDDEN              = 0x00000002
    FILE_ATTRIBUTE_SYSTEM              = 0x00000004
    FILE_ATTRIBUTE_DIRECTORY           = 0x00000010
    FILE_ATTRIBUTE_ARCHIVE             = 0x00000020
    FILE_ATTRIBUTE_DEVICE              = 0x00000040
    FILE_ATTRIBUTE_NORMAL              = 0x00000080
    FILE_ATTRIBUTE_TEMPORARY           = 0x00000100
    FILE_ATTRIBUTE_SPARSE_FILE         = 0x00000200
    FILE_ATTRIBUTE_REPARSE_POINT       = 0x00000400
    FILE_ATTRIBUTE_COMPRESSED          = 0x00000800
    FILE_ATTRIBUTE_OFFLINE             = 0x00001000
    FILE_ATTRIBUTE_NOT_CONTENT_INDEXED = 0x00002000
    FILE_ATTRIBUTE_ENCRYPTED           = 0x00004000
)

type WindowsAttributes struct {
    Attributes uint32
}

func (wa *WindowsAttributes) IsHidden() bool {
    return wa.Attributes & FILE_ATTRIBUTE_HIDDEN != 0
}

func (wa *WindowsAttributes) IsSystem() bool {
    return wa.Attributes & FILE_ATTRIBUTE_SYSTEM != 0
}

func (wa *WindowsAttributes) IsReadOnly() bool {
    return wa.Attributes & FILE_ATTRIBUTE_READONLY != 0
}

// Extended file info with Windows attributes
type FileInfoWithAttrs struct {
    fs.FileInfo
    WinAttrs *WindowsAttributes
}
```

### Opportunistic Locking (Oplocks)

```go
// Oplock levels
const (
    OPLOCK_LEVEL_NONE       = 0x00
    OPLOCK_LEVEL_II         = 0x01  // Shared oplock
    OPLOCK_LEVEL_EXCLUSIVE  = 0x08  // Exclusive oplock
    OPLOCK_LEVEL_BATCH      = 0x09  // Batch oplock
    OPLOCK_LEVEL_LEASE      = 0xFF  // SMB3 lease (durable)
)

type OplockManager struct {
    mu      sync.Mutex
    oplocks map[uint64]*Oplock  // File ID -> Oplock
}

type Oplock struct {
    FileID      uint64
    Level       uint8
    BreakNotify chan struct{}
}

// Request oplock on file open
func (om *OplockManager) RequestOplock(fileID uint64, level uint8) (*Oplock, error) {
    oplock := &Oplock{
        FileID:      fileID,
        Level:       level,
        BreakNotify: make(chan struct{}, 1),
    }

    om.mu.Lock()
    om.oplocks[fileID] = oplock
    om.mu.Unlock()

    return oplock, nil
}

// Handle oplock break notification from server
func (om *OplockManager) HandleBreak(fileID uint64, newLevel uint8) {
    om.mu.Lock()
    oplock, exists := om.oplocks[fileID]
    om.mu.Unlock()

    if !exists {
        return
    }

    // Notify application of break
    select {
    case oplock.BreakNotify <- struct{}{}:
    default:
    }

    // Flush cached data, acknowledge break, etc.
}
```

**Oplock benefits:**
- Reduced server round-trips for cached data
- Client-side caching of file data and metadata
- Better performance for read-heavy workloads
- Automatic cache invalidation on server changes

### Large File Support (>4GB)

```go
// Ensure 64-bit file operations
type LargeFileSupport struct {
    capabilities uint32
}

const (
    CAP_LARGE_FILES     = 0x00000008
    CAP_LARGE_READX     = 0x00004000
    CAP_LARGE_WRITEX    = 0x00008000
)

func (lfs *LargeFileSupport) MaxReadSize() int64 {
    if lfs.capabilities & CAP_LARGE_READX != 0 {
        return 16 * 1024 * 1024  // 16MB max read
    }
    return 64 * 1024  // 64KB standard
}

func (lfs *LargeFileSupport) MaxWriteSize() int64 {
    if lfs.capabilities & CAP_LARGE_WRITEX != 0 {
        return 16 * 1024 * 1024  // 16MB max write
    }
    return 64 * 1024  // 64KB standard
}

// Split large I/O into chunks
func (f *File) Read(p []byte) (n int, err error) {
    maxChunk := f.fs.largeFile.MaxReadSize()

    for n < len(p) {
        chunkSize := min(int64(len(p)-n), maxChunk)

        chunk, err := f.readChunk(f.offset+int64(n), int(chunkSize))
        if err != nil {
            if err == io.EOF && n > 0 {
                return n, nil
            }
            return n, err
        }

        copy(p[n:], chunk)
        n += len(chunk)

        if len(chunk) < int(chunkSize) {
            return n, io.EOF
        }
    }

    return n, nil
}
```

### Error Handling and Retry Logic

```go
type RetryPolicy struct {
    MaxAttempts int
    InitialDelay time.Duration
    MaxDelay     time.Duration
    Multiplier   float64
}

var defaultRetryPolicy = RetryPolicy{
    MaxAttempts:  3,
    InitialDelay: 100 * time.Millisecond,
    MaxDelay:     5 * time.Second,
    Multiplier:   2.0,
}

// Retryable error types
func isRetryable(err error) bool {
    if err == nil {
        return false
    }

    // Network errors are retryable
    var netErr net.Error
    if errors.As(err, &netErr) && netErr.Temporary() {
        return true
    }

    // SMB status codes that are retryable
    var smbErr *SMBError
    if errors.As(err, &smbErr) {
        switch smbErr.Status {
        case STATUS_NETWORK_NAME_DELETED,
             STATUS_CONNECTION_DISCONNECTED,
             STATUS_CONNECTION_RESET,
             STATUS_INSUFF_SERVER_RESOURCES:
            return true
        }
    }

    return false
}

func (fs *FileSystem) withRetry(ctx context.Context, op func() error) error {
    policy := fs.config.RetryPolicy
    if policy == nil {
        policy = &defaultRetryPolicy
    }

    var lastErr error
    delay := policy.InitialDelay

    for attempt := 0; attempt < policy.MaxAttempts; attempt++ {
        // Check context cancellation
        if ctx.Err() != nil {
            return ctx.Err()
        }

        // Attempt operation
        err := op()
        if err == nil {
            return nil
        }

        lastErr = err

        // Don't retry if error is not retryable
        if !isRetryable(err) {
            return err
        }

        // Don't retry on last attempt
        if attempt == policy.MaxAttempts-1 {
            break
        }

        // Exponential backoff
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(delay):
        }

        delay = time.Duration(float64(delay) * policy.Multiplier)
        if delay > policy.MaxDelay {
            delay = policy.MaxDelay
        }
    }

    return lastErr
}
```

### Timeout Configuration

```go
type TimeoutConfig struct {
    // Connection timeouts
    ConnectTimeout time.Duration  // TCP connection (default: 30s)
    NegotiateTimeout time.Duration // Dialect negotiation (default: 10s)
    AuthTimeout time.Duration      // Authentication (default: 30s)

    // Operation timeouts
    ReadTimeout  time.Duration     // Read operations (default: 60s)
    WriteTimeout time.Duration     // Write operations (default: 60s)
    StatTimeout  time.Duration     // Metadata operations (default: 10s)

    // Session timeouts
    IdleTimeout    time.Duration   // Idle connection (default: 5m)
    SessionTimeout time.Duration   // Total session lifetime (default: 24h)
    KeepAlive      time.Duration   // Keep-alive interval (default: 60s)
}

// Apply timeout to context
func (tc *TimeoutConfig) withTimeout(ctx context.Context, op string) (context.Context, context.CancelFunc) {
    var timeout time.Duration

    switch op {
    case "read":
        timeout = tc.ReadTimeout
    case "write":
        timeout = tc.WriteTimeout
    case "stat":
        timeout = tc.StatTimeout
    default:
        timeout = tc.ReadTimeout  // default
    }

    return context.WithTimeout(ctx, timeout)
}
```

## Implementation Phases

### Phase 1: Core Infrastructure
- SMB client library integration (go-smb2)
- Connection management and pooling
- Basic authentication (username/password, NTLM)
- Session lifecycle management
- Configuration parsing and validation
- Error handling framework

### Phase 2: Basic File Operations
- File open/close/read/write
- File stat and metadata
- Basic directory operations (ReadDir, Mkdir, Remove)
- Path normalization and validation
- absfs.FileSystem interface implementation
- Unit tests for core operations

### Phase 3: Advanced Features
- Advanced authentication (Kerberos, domain)
- Permission handling (ACL mapping)
- Windows attribute support
- Oplock management
- Large file support (>4GB)
- Share enumeration

### Phase 4: Performance Optimization
- Connection pooling optimization
- Directory metadata caching
- Parallel operations where possible
- Buffer size tuning
- Batch operations
- Performance benchmarking

### Phase 5: Production Readiness
- Comprehensive error handling
- Retry logic and resilience
- Timeout configuration
- Logging and debugging
- Integration tests
- Documentation and examples
- Security audit

## Usage Examples

### Connect to Windows Share

```go
package main

import (
    "context"
    "fmt"
    "io/fs"
    "log"

    "github.com/absfs/smbfs"
)

func main() {
    // Connect to Windows file share
    fsys, err := smbfs.New(&smbfs.Config{
        Server:   "fileserver.corp.example.com",
        Share:    "departments",
        Username: "jdoe",
        Password: "secret123",
        Domain:   "CORP",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer fsys.Close()

    // List files in engineering directory
    entries, err := fs.ReadDir(fsys, "/engineering")
    if err != nil {
        log.Fatal(err)
    }

    for _, entry := range entries {
        info, _ := entry.Info()
        fmt.Printf("%s %10d %s\n",
            info.Mode(),
            info.Size(),
            entry.Name())
    }

    // Read a file
    data, err := fs.ReadFile(fsys, "/engineering/specs/design.pdf")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Read %d bytes\n", len(data))
}
```

### Connect to Samba Server

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/absfs/smbfs"
)

func main() {
    // Connect to Samba server (Linux/Unix)
    fsys, err := smbfs.New(&smbfs.Config{
        Server:   "nas.home.local",
        Share:    "media",
        Username: "homeuser",
        Password: "homepass",
        // No domain needed for Samba workgroup
    })
    if err != nil {
        log.Fatal(err)
    }
    defer fsys.Close()

    // Upload a video file
    ctx := context.Background()
    src, err := os.Open("/local/videos/movie.mp4")
    if err != nil {
        log.Fatal(err)
    }
    defer src.Close()

    dst, err := fsys.OpenFile("/videos/uploaded/movie.mp4",
        os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        log.Fatal(err)
    }
    defer dst.Close()

    written, err := io.Copy(dst, src)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Uploaded %d bytes\n", written)
}
```

### Domain-Joined Authentication

```go
package main

import (
    "log"
    "os"

    "github.com/absfs/smbfs"
)

func main() {
    // Use Kerberos authentication (requires kinit)
    fsys, err := smbfs.New(&smbfs.Config{
        Server:      "fileserver.corp.example.com",
        Share:       "home",
        UseKerberos: true,
        Domain:      "CORP",
        Username:    os.Getenv("USER"),
    })
    if err != nil {
        log.Fatal(err)
    }
    defer fsys.Close()

    // Access home directory with Kerberos ticket
    // No password needed - uses Kerberos ticket cache

    entries, err := fsys.ReadDir("/")
    if err != nil {
        log.Fatal(err)
    }

    for _, entry := range entries {
        log.Printf("  %s\n", entry.Name())
    }
}
```

### Composition with cachefs for Performance

```go
package main

import (
    "log"
    "time"

    "github.com/absfs/absfs"
    "github.com/absfs/cachefs"
    "github.com/absfs/smbfs"
    "github.com/absfs/memfs"
)

func main() {
    // Create SMB filesystem
    remote, err := smbfs.New(&smbfs.Config{
        Server:   "fileserver.corp.example.com",
        Share:    "projects",
        Username: "jdoe",
        Password: "secret123",
        Domain:   "CORP",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer remote.Close()

    // Create in-memory cache
    cache := memfs.New()

    // Compose with cachefs for better performance
    fsys, err := cachefs.New(&cachefs.Config{
        Remote: remote,
        Cache:  cache,
        TTL:    5 * time.Minute,
        // Cache metadata and small files
        MaxCacheSize: 100 * 1024 * 1024, // 100MB
        CachePolicy:  cachefs.PolicyLRU,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer fsys.Close()

    // Now operations are cached
    // First read hits network
    data1, _ := fs.ReadFile(fsys, "/project/README.md")

    // Second read hits cache (much faster)
    data2, _ := fs.ReadFile(fsys, "/project/README.md")

    log.Printf("Read %d bytes (cached: %d)\n", len(data1), len(data2))
}
```

## Platform Support

smbfs is a cross-platform SMB client implementation:

**Supported Platforms:**
- **Linux** - Full support, native performance
- **macOS** - Full support, native performance
- **Windows** - Full support (alternative to built-in SMB)
- **FreeBSD** - Full support with go-smb2
- **Other Unix** - Should work on any platform with Go support

**Platform-Specific Features:**
- **Linux**: Integration with system Kerberos (`krb5.conf`)
- **macOS**: Keychain integration for credential storage
- **Windows**: Can use Windows credential manager
- **All**: Pure Go implementation, no CGO required

**Advantages over OS-native SMB:**
- Consistent behavior across platforms
- No dependency on OS SMB stack
- Programmatic control over connection parameters
- Better error handling and debugging
- Composable with other absfs implementations

## Performance Considerations

### Network Latency
- High latency impacts small operations (stat, readdir)
- Use caching (cachefs) for metadata-heavy workloads
- Batch operations when possible
- Consider directory caching for repeated scans

### Bandwidth Optimization
- Large buffer sizes for sequential I/O (default: 64KB)
- Connection reuse via pooling
- Parallel transfers for multiple files
- SMB3 multichannel support (future)

### Connection Pooling
- Shared connections reduce auth overhead
- Idle timeout prevents resource exhaustion
- Max connections limit controls server load
- Thread-safe connection management

### Caching Strategies
- Metadata caching (file info, directory listings)
- Read-ahead for sequential access
- Write-behind for improved write performance
- Oplock-based cache coherency

**Benchmarking:**

```go
// Benchmark sequential reads
func BenchmarkSequentialRead(b *testing.B) {
    fsys := setupSMBFS()
    defer fsys.Close()

    f, _ := fsys.Open("/testfile")
    defer f.Close()

    buf := make([]byte, 64*1024)
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        f.Read(buf)
    }
}

// Benchmark with vs without caching
func BenchmarkCachedVsUncached(b *testing.B) {
    // Test both scenarios
}
```

## Testing Strategy

### Unit Tests
- Connection management
- Authentication methods
- Path normalization
- Error handling
- Configuration parsing
- Pool behavior

### Integration Tests
- Real SMB server connectivity
- File operations (CRUD)
- Directory operations
- Large file transfers
- Concurrent operations
- Session recovery

### Test Infrastructure

```go
// Test server setup
type TestSMBServer struct {
    Server   string
    Share    string
    Username string
    Password string
}

func SetupTestServer(t *testing.T) *TestSMBServer {
    // Use Docker to run Samba container
    // Or connect to pre-configured test server
}

func TestBasicOperations(t *testing.T) {
    srv := SetupTestServer(t)

    fsys, err := smbfs.New(&smbfs.Config{
        Server:   srv.Server,
        Share:    srv.Share,
        Username: srv.Username,
        Password: srv.Password,
    })
    require.NoError(t, err)
    defer fsys.Close()

    // Test file creation
    f, err := fsys.Create("/test.txt")
    require.NoError(t, err)

    _, err = f.Write([]byte("hello world"))
    require.NoError(t, err)

    f.Close()

    // Test file reading
    data, err := fs.ReadFile(fsys, "/test.txt")
    require.NoError(t, err)
    assert.Equal(t, "hello world", string(data))

    // Test file deletion
    err = fsys.Remove("/test.txt")
    require.NoError(t, err)
}
```

### Compatibility Testing
- Windows Server (2012, 2016, 2019, 2022)
- Samba versions (4.x)
- NAS devices (Synology, QNAP, etc.)
- Different SMB dialects
- Various authentication methods

### Performance Testing
- Throughput benchmarks
- Latency measurements
- Connection pool efficiency
- Cache effectiveness
- Concurrent operation scaling

## Comparison with Other Network Filesystems

| Feature | smbfs | webdavfs | sftpfs | httpfs |
|---------|-------|----------|--------|--------|
| **Protocol** | SMB/CIFS | WebDAV | SSH/SFTP | HTTP/HTTPS |
| **Primary Use** | Windows shares | Web storage | SSH servers | Web servers |
| **Authentication** | NTLM, Kerberos, Domain | Basic, Digest, OAuth | SSH keys, password | Basic, Bearer |
| **Write Support** | Full | Full | Full | Limited |
| **Performance** | High (LAN) | Medium | Medium | Medium |
| **Firewall Friendly** | Port 445 (often blocked) | Port 80/443 (open) | Port 22 (often open) | Port 80/443 (open) |
| **Windows Integration** | Native | Good | Limited | Limited |
| **Unix Integration** | Good | Good | Native | Limited |
| **Large Files** | Excellent | Good | Good | Limited |
| **Encryption** | SMB3+ | TLS | SSH | TLS |
| **Permissions** | Windows ACL | WebDAV ACL | Unix permissions | Limited |
| **Locking** | Oplocks | WebDAV locks | Advisory | None |
| **Caching** | Client-side | Client-side | Limited | ETags |

**When to use smbfs:**
- Accessing Windows file shares
- Corporate network file storage
- Samba servers in mixed environments
- NAS devices with SMB support
- Integration with Active Directory
- High-performance LAN file access

**When to use alternatives:**
- **webdavfs**: Internet-accessible storage, cloud services
- **sftpfs**: SSH-enabled servers, Unix environments
- **httpfs**: Read-only web content, static files

**Composition patterns:**

```go
// SMB with caching for WAN scenarios
cachedSMB := cachefs.New(smbfs.New(...), memfs.New())

// SMB with retry for unreliable networks
resilientSMB := retryfs.New(smbfs.New(...))

// SMB with metrics for monitoring
monitoredSMB := metricsfs.New(smbfs.New(...))

// SMB with encryption layer (additional to SMB3 encryption)
securedSMB := encryptfs.New(smbfs.New(...))
```

## Security Considerations

### Network Security
- Always use SMB3+ with encryption for untrusted networks
- Enable message signing to prevent tampering
- Use VPN for Internet-exposed SMB servers
- Avoid SMB1 (disabled by default)

### Credential Security
- Store credentials in system keyring, not plaintext
- Use Kerberos for domain environments (no password transmission)
- Rotate passwords regularly
- Use least-privilege accounts

### Access Control
- Respect Windows ACLs
- Map permissions appropriately
- Audit access logs
- Implement client-side validation

### Attack Mitigation
- Rate limiting for failed authentication
- Connection timeout to prevent resource exhaustion
- Input validation for paths and filenames
- Protection against path traversal

## License

MIT License - see LICENSE file for details

## Contributing

Contributions welcome! Please see CONTRIBUTING.md for guidelines.

## Documentation

### Getting Started
- **[README.md](README.md)** - This file (overview and quick start)
- **[examples/basic](examples/basic/)** - Basic usage examples
- **[examples/advanced](examples/advanced/)** - Advanced features demonstration
- **[examples/caching](examples/caching/)** - Performance optimization with caching
- **[examples/windows-attributes](examples/windows-attributes/)** - Windows file attributes

### Operations & Deployment
- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Production deployment guide
- **[PERFORMANCE.md](PERFORMANCE.md)** - Performance optimization guide
- **[TROUBLESHOOTING.md](TROUBLESHOOTING.md)** - Problem diagnosis and solutions
- **[TESTING.md](TESTING.md)** - Testing guide (unit, integration, benchmarks)

### Reference
- **[PROJECT_STATUS.md](PROJECT_STATUS.md)** - Implementation status and roadmap
- **[SECURITY.md](SECURITY.md)** - Security audit (APPROVED) and best practices
- **[KERBEROS.md](KERBEROS.md)** - Kerberos authentication setup guide
- **[API Documentation](https://pkg.go.dev/github.com/absfs/smbfs)** - Complete godoc reference

## Related Projects

- [absfs](https://github.com/absfs/absfs) - Core filesystem abstraction
- [cachefs](https://github.com/absfs/cachefs) - Caching filesystem wrapper
- [webdavfs](https://github.com/absfs/webdavfs) - WebDAV filesystem
- [sftpfs](https://github.com/absfs/sftpfs) - SFTP filesystem
- [httpfs](https://github.com/absfs/httpfs) - HTTP filesystem
- [go-smb2](https://github.com/hirochachacha/go-smb2) - SMB2/3 client library

## Support

- Issues: https://github.com/absfs/smbfs/issues
- Discussions: https://github.com/absfs/smbfs/discussions
- Documentation: https://pkg.go.dev/github.com/absfs/smbfs
