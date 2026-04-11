# Troubleshooting Guide

Quick reference for diagnosing and resolving common smbfs issues.

## Table of Contents

- [Connection Issues](#connection-issues)
- [Authentication Problems](#authentication-problems)
- [Performance Issues](#performance-issues)
- [File Operation Errors](#file-operation-errors)
- [Cache Issues](#cache-issues)
- [Debugging Tips](#debugging-tips)

---

## Connection Issues

### Error: "connection refused"

**Symptoms:** Cannot connect to SMB server

**Causes:**
- SMB server not running
- Incorrect server address/port
- Firewall blocking connection
- Network connectivity issues

**Solutions:**

```bash
# 1. Verify server is reachable
ping server.example.com

# 2. Check SMB port is open
telnet server.example.com 445
nc -zv server.example.com 445

# 3. Try alternative port (older SMB)
telnet server.example.com 139
```

```go
// 4. Verify configuration
config := &smbfs.Config{
    Server: "server.example.com",  // Not "\\server" or "smb://server"
    Port:   445,                    // Default SMB port
    Share:  "sharename",            // Not "\\server\share"
}
```

### Error: "i/o timeout"

**Symptoms:** Operations time out

**Causes:**
- Network latency
- Server overloaded
- Timeout too short
- Connection pool exhausted

**Solutions:**

```go
config := &smbfs.Config{
    ConnTimeout: 60 * time.Second,  // Increase from default 30s
    OpTimeout:   120 * time.Second, // Increase from default 60s
    MaxOpen:     50,                // Increase if hitting pool limits
}
```

### Error: "connection pool exhausted"

**Symptoms:** "connection pool exhausted" in logs

**Causes:**
- Too many concurrent operations
- MaxOpen too small
- Connections not being closed

**Solutions:**

```go
// 1. Increase pool size
config.MaxOpen = 50

// 2. Ensure files are closed
file, err := fsys.Open("/file.txt")
if err != nil {
    return err
}
defer file.Close()  // Don't forget this!

// 3. Check for connection leaks
// Use logging to track open/close operations
```

---

## Authentication Problems

### Error: "authentication failed"

**Symptoms:** Cannot authenticate to server

**Causes:**
- Wrong username/password
- Domain not specified
- Incorrect domain format
- Account locked/expired

**Solutions:**

```go
// 1. Basic auth
config := &smbfs.Config{
    Username: "myuser",
    Password: "mypassword",
}

// 2. Domain user (backslash format)
config := &smbfs.Config{
    Username: "DOMAIN\\user",  // or
    Domain:   "DOMAIN",
    Username: "user",
}

// 3. Domain user (@ format)
config := &smbfs.Config{
    Username: "user@DOMAIN.COM",
}

// 4. Guest access
config := &smbfs.Config{
    GuestAccess: true,
}
```

```bash
# 5. Test credentials manually
smbclient //server/share -U username
```

### Error: "permission denied"

**Symptoms:** Cannot access files despite authentication

**Causes:**
- Insufficient share permissions
- File/directory ACLs
- Read-only share

**Solutions:**

```bash
# 1. Check share permissions on server
# 2. Verify user has required rights
# 3. Check file/folder permissions
```

```go
// 4. Try read-only operations first
info, err := fsys.Stat("/file.txt")  // Read operation
if err != nil {
    log.Fatal("Can't even read:", err)
}

// Then try write
err = fsys.Remove("/file.txt")  // Write operation
if err != nil {
    log.Fatal("No write permissions:", err)
}
```

---

## Performance Issues

### Slow File Operations

**Symptoms:** Individual operations take too long

**Diagnosis:**

```go
import "time"

start := time.Now()
info, err := fsys.Stat("/file.txt")
duration := time.Since(start)
log.Printf("Stat took %v", duration)

// Normal: < 10ms on LAN
// Slow: > 100ms
```

**Solutions:**

```go
// 1. Enable caching
config.Cache.EnableCache = true
config.Cache.StatCacheTTL = 10 * time.Second

// 2. Adjust buffer sizes
config.ReadBufferSize = 256 * 1024  // For large files
config.WriteBufferSize = 256 * 1024

// 3. Increase connection pool
config.MaxIdle = 10
config.MaxOpen = 20

// 4. Reduce network latency
// - Use wired connection
// - Check for packet loss: mtr server.example.com
// - Verify MTU settings
```

### High Memory Usage

**Symptoms:** Process uses too much memory

**Diagnosis:**

```bash
# Monitor memory usage
ps aux | grep your-app
top -p $(pgrep your-app)
```

**Solutions:**

```go
// 1. Reduce cache size
config.Cache.MaxCacheEntries = 1000  // From 5000

// 2. Reduce buffer sizes
config.ReadBufferSize = 64 * 1024   // From 256KB
config.WriteBufferSize = 64 * 1024

// 3. Reduce connection pool
config.MaxIdle = 5  // From 10
config.MaxOpen = 10 // From 20

// 4. Ensure files are closed
// Check for file descriptor leaks
```

---

## File Operation Errors

### Error: "file not found"

**Symptoms:** Cannot find existing file

**Causes:**
- Path format incorrect
- Case sensitivity issues
- File doesn't exist

**Solutions:**

```go
// 1. Use Unix-style paths
path := "/dir/file.txt"  // ✓ Correct
path := "\\dir\\file.txt"  // ✗ Wrong

// 2. Check case sensitivity
config.CaseSensitive = false  // For Windows servers (default)
config.CaseSensitive = true   // For case-sensitive shares

// 3. List directory to see actual names
entries, _ := fsys.ReadDir("/")
for _, entry := range entries {
    log.Println(entry.Name())  // See exact names
}

// 4. Use absolute paths
path = fsys.pathNorm.normalize(path)
```

### Error: "file already exists"

**Symptoms:** Cannot create file that already exists

**Solutions:**

```go
// 1. Check if file exists first
_, err := fsys.Stat("/file.txt")
if err == nil {
    // File exists, remove it first
    fsys.Remove("/file.txt")
}

// 2. Or use OpenFile with appropriate flags
file, err := fsys.OpenFile("/file.txt",
    os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)

// 3. For directories
err := fsys.MkdirAll("/path/to/dir", 0755)  // Won't error if exists
```

---

## Cache Issues

### Stale Cache Data

**Symptoms:** Old file data returned

**Causes:**
- Cache TTL too long
- Multi-client modifications
- Cache not invalidated

**Solutions:**

```go
// 1. Reduce TTL
config.Cache.StatCacheTTL = 5 * time.Second  // From 30s

// 2. Disable cache for write-heavy workloads
config.Cache.EnableCache = false

// 3. Use short TTLs in multi-client scenarios
config.Cache.DirCacheTTL = 2 * time.Second
config.Cache.StatCacheTTL = 2 * time.Second
```

### Cache Not Helping Performance

**Symptoms:** No performance improvement with caching

**Diagnosis:**

```go
// Check cache stats (if available in future version)
// For now, time operations with/without cache

// Without cache
config1 := &smbfs.Config{...}
config1.Cache.EnableCache = false
fsys1, _ := smbfs.New(config1)

start := time.Now()
for i := 0; i < 100; i++ {
    fsys1.Stat("/file.txt")
}
fmt.Println("Without cache:", time.Since(start))

// With cache
config2 := &smbfs.Config{...}
config2.Cache.EnableCache = true
fsys2, _ := smbfs.New(config2)

start = time.Now()
for i := 0; i < 100; i++ {
    fsys2.Stat("/file.txt")
}
fmt.Println("With cache:", time.Since(start))
```

**Possible Causes:**

1. **Cache disabled**: Check `config.Cache.EnableCache = true`
2. **TTL too short**: Operations complete before next access
3. **Different paths**: Cache is path-specific
4. **Write operations**: Invalidate cache frequently

---

## Debugging Tips

### Enable Debug Logging

```go
type DebugLogger struct{}

func (l *DebugLogger) Printf(format string, v ...interface{}) {
    log.Printf("[SMBFS] "+format, v...)
}

config.Logger = &DebugLogger{}
```

### Use Network Tools

```bash
# Monitor SMB traffic
sudo tcpdump -i eth0 'port 445'

# Check connectivity
ping server.example.com
traceroute server.example.com
mtr server.example.com

# Test SMB connection
smbclient //server/share -U username

# Check DNS resolution
nslookup server.example.com
dig server.example.com
```

### Common Checks

```go
// 1. Validate configuration
if err := config.Validate(); err != nil {
    log.Fatal("Config error:", err)
}

// 2. Test basic operation
info, err := fsys.Stat("/")
if err != nil {
    log.Fatal("Can't stat root:", err)
}
log.Println("Connected successfully, root exists")

// 3. Check version compatibility
// SMB 2.0+ required
// Server must support modern SMB

// 4. Test with simple operations first
entries, err := fsys.ReadDir("/")
if err != nil {
    log.Fatal("Can't list root:", err)
}
log.Printf("Root has %d entries", len(entries))
```

### Error Messages Reference

| Error | Meaning | Common Fix |
|-------|---------|------------|
| connection refused | Server not reachable | Check server/port/firewall |
| authentication failed | Wrong credentials | Check username/password/domain |
| permission denied | Insufficient rights | Check share/file permissions |
| file not found | Path incorrect | Check path format and case |
| i/o timeout | Operation took too long | Increase timeouts |
| connection closed | Connection lost | Check network stability |
| pool exhausted | Too many operations | Increase MaxOpen |

### Getting Help

When reporting issues, include:

1. **smbfs version**: Check go.mod
2. **Go version**: `go version`
3. **OS**: Linux/Windows/macOS
4. **Server type**: Windows/Samba/NAS
5. **Error message**: Full error with stack trace
6. **Configuration**: Sanitized config (no passwords!)
7. **Minimal reproduction**: Smallest code that shows the issue

```go
// Example bug report setup
package main

import (
    "log"
    "github.com/absfs/smbfs"
)

func main() {
    config := &smbfs.Config{
        Server:   "server",
        Share:    "share",
        Username: "user",
        Password: "***",  // Never include real password!
    }

    fsys, err := smbfs.New(config)
    if err != nil {
        log.Fatal(err)  // Include this error in report
    }
    defer fsys.Close()

    // Minimal code that reproduces the issue
    _, err = fsys.Stat("/problematic/path")
    log.Fatal(err)  // Include this error too
}
```

---

For more help:
- Check [PERFORMANCE.md](PERFORMANCE.md) for optimization tips
- Review [SECURITY.md](SECURITY.md) for security best practices
- See [examples/](examples/) for working code samples
- Report issues: https://github.com/absfs/smbfs/issues
