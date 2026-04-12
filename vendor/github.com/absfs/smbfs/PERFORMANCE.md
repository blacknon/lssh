# Performance Guide

This guide explains how to optimize smbfs performance for your specific use case.

## Table of Contents

- [Performance Overview](#performance-overview)
- [Caching](#caching)
- [Connection Pooling](#connection-pooling)
- [Buffer Sizes](#buffer-sizes)
- [Retry Strategy](#retry-strategy)
- [Best Practices](#best-practices)
- [Benchmarking](#benchmarking)
- [Troubleshooting Performance Issues](#troubleshooting-performance-issues)

---

## Performance Overview

smbfs is designed for high performance with several optimization features:

- **Metadata caching**: Reduces redundant network calls for file/directory info
- **Connection pooling**: Reuses SMB connections across operations
- **Configurable buffers**: Optimizes I/O for different file sizes
- **Retry with backoff**: Handles transient network issues efficiently

### Performance Characteristics

Based on benchmark testing:

| Operation | Typical Performance |
|-----------|-------------------|
| File creation | < 10ms per file |
| Small file read (1KB) | < 5ms |
| Medium file read (64KB) | < 20ms |
| Large file read (1MB) | < 100ms |
| Directory listing | < 50ms (100 entries) |
| Metadata operations | < 5ms (cached), < 15ms (uncached) |

*Note: Actual performance depends on network latency, server load, and file sizes.*

---

## Caching

### Metadata Caching

Metadata caching significantly improves performance for repeated Stat() and ReadDir() operations.

**Configuration:**

```go
config := &smbfs.Config{
    Server:   "server.example.com",
    Share:    "myshare",
    Username: "user",
    Password: "pass",

    // Enable metadata caching
    Cache: smbfs.CacheConfig{
        EnableCache:     true,              // Enable caching
        DirCacheTTL:     10 * time.Second,  // Directory listing cache TTL
        StatCacheTTL:    10 * time.Second,  // File stat cache TTL
        MaxCacheEntries: 5000,              // Maximum cached entries
    },
}
```

**When to Enable Caching:**

✅ **Good for:**
- Read-heavy workloads
- Repeated access to same files/directories
- Directory trees with slow-changing metadata
- Applications that traverse directory hierarchies frequently

❌ **Not recommended for:**
- Write-heavy workloads
- Rapidly changing files
- Multi-client scenarios (cache may be stale)
- Strict consistency requirements

**Cache TTL Guidelines:**

| Use Case | DirCacheTTL | StatCacheTTL |
|----------|-------------|--------------|
| Static content | 60s - 300s | 60s - 300s |
| Mostly read | 10s - 30s | 10s - 30s |
| Mixed read/write | 5s - 10s | 5s - 10s |
| Write-heavy | Disable | Disable |

**Performance Impact:**

- **Cached Stat()**: 70-90% reduction in latency
- **Cached ReadDir()**: 80-95% reduction in latency
- **Memory usage**: ~500 bytes per cached entry
- **CPU overhead**: Negligible

**Example:**

```go
// Without caching
for i := 0; i < 1000; i++ {
    info, _ := fsys.Stat("/file.txt")  // ~15ms each = 15 seconds total
}

// With caching (5s TTL)
for i := 0; i < 1000; i++ {
    info, _ := fsys.Stat("/file.txt")  // ~15ms first, ~1ms rest = 1 second total
}
```

### Cache Invalidation

The cache is automatically invalidated when you:
- Create files (`Create()`, `OpenFile()` with `O_CREATE`)
- Delete files (`Remove()`, `RemoveAll()`)
- Rename files (`Rename()`)
- Modify metadata (`Chmod()`, `Chtimes()`)
- Create directories (`Mkdir()`, `MkdirAll()`)

**Manual Cache Control:**

Currently, there's no API to manually flush the cache. If you need this for your use case, consider:
1. Creating a new filesystem instance (starts fresh)
2. Setting very short TTLs (1-2 seconds)
3. Disabling cache for specific operations

---

## Connection Pooling

Connection pooling reduces the overhead of establishing SMB connections.

**Configuration:**

```go
config := &smbfs.Config{
    Server:   "server.example.com",
    Share:    "myshare",
    Username: "user",
    Password: "pass",

    // Connection pool settings
    MaxIdle:     10,              // Max idle connections (default: 5)
    MaxOpen:     20,              // Max open connections (default: 10)
    IdleTimeout: 5 * time.Minute, // Idle connection timeout (default: 5m)
    ConnTimeout: 30 * time.Second,// Connection timeout (default: 30s)
}
```

**Pool Size Guidelines:**

| Workload Type | MaxIdle | MaxOpen |
|---------------|---------|---------|
| Single-threaded | 2-3 | 5-10 |
| Light concurrent | 5-10 | 10-20 |
| Heavy concurrent | 10-20 | 20-50 |
| Very high concurrency | 20-50 | 50-100 |

**Recommendations:**

1. **MaxOpen**: Set to expected concurrent operations + 20%
2. **MaxIdle**: Set to 50-75% of MaxOpen to balance connection reuse and resource usage
3. **IdleTimeout**: 5-10 minutes for most use cases
4. **ConnTimeout**: 30 seconds for LAN, 60 seconds for WAN

**Performance Impact:**

- **Connection reuse**: 90-95% reduction in connection establishment overhead
- **Memory per connection**: ~1-2 MB
- **CPU overhead**: Minimal

**Example:**

```go
// Poor: Single connection, high overhead
config := &smbfs.Config{
    MaxIdle: 1,
    MaxOpen: 1,  // Every operation waits for the single connection
}

// Better: Moderate pool for concurrent operations
config := &smbfs.Config{
    MaxIdle: 5,
    MaxOpen: 10,  // Up to 10 concurrent operations
}

// Best for high concurrency: Large pool
config := &smbfs.Config{
    MaxIdle: 20,
    MaxOpen: 50,  // Handles 50 concurrent operations
}
```

---

## Buffer Sizes

Buffer sizes affect I/O performance, especially for large files.

**Configuration:**

```go
config := &smbfs.Config{
    Server:   "server.example.com",
    Share:    "myshare",
    Username: "user",
    Password: "pass",

    ReadBufferSize:  128 * 1024,  // 128 KB read buffer (default: 64KB)
    WriteBufferSize: 128 * 1024,  // 128 KB write buffer (default: 64KB)
}
```

**Buffer Size Guidelines:**

| File Size | Read Buffer | Write Buffer |
|-----------|-------------|--------------|
| Small (< 1MB) | 32-64 KB | 32-64 KB |
| Medium (1-100MB) | 128-256 KB | 128-256 KB |
| Large (> 100MB) | 256-512 KB | 256-512 KB |
| Very Large (> 1GB) | 512 KB - 1 MB | 512 KB - 1 MB |

**Recommendations:**

1. **Network speed matters**: Larger buffers for faster networks (1 Gbps+)
2. **Memory trade-off**: Larger buffers use more memory
3. **Server capabilities**: Some servers have max transfer sizes
4. **Testing**: Benchmark your specific workload

**Performance Impact:**

| Buffer Size | Large File Throughput | Memory per Operation |
|-------------|---------------------|---------------------|
| 32 KB | 50-100 MB/s | 32 KB |
| 64 KB (default) | 100-200 MB/s | 64 KB |
| 128 KB | 150-300 MB/s | 128 KB |
| 256 KB | 200-400 MB/s | 256 KB |
| 512 KB | 250-500 MB/s | 512 KB |

*Actual throughput depends on network speed, latency, and server performance.*

**Example:**

```go
// Small files: Optimize for low latency
config := &smbfs.Config{
    ReadBufferSize:  32 * 1024,   // 32 KB
    WriteBufferSize: 32 * 1024,   // 32 KB
}

// Large files: Optimize for throughput
config := &smbfs.Config{
    ReadBufferSize:  512 * 1024,  // 512 KB
    WriteBufferSize: 512 * 1024,  // 512 KB
}
```

---

## Retry Strategy

Retry configuration affects reliability and performance under network issues.

**Configuration:**

```go
config := &smbfs.Config{
    Server:   "server.example.com",
    Share:    "myshare",
    Username: "user",
    Password: "pass",

    // Custom retry policy
    RetryPolicy: &smbfs.RetryPolicy{
        MaxAttempts:  5,                 // Max retry attempts (default: 3)
        InitialDelay: 100 * time.Millisecond, // Initial delay (default: 100ms)
        MaxDelay:     5 * time.Second,   // Max delay (default: 10s)
        Multiplier:   2.0,               // Backoff multiplier (default: 2.0)
    },
}
```

**Retry Guidelines:**

| Network Quality | MaxAttempts | InitialDelay | MaxDelay |
|-----------------|-------------|--------------|----------|
| Reliable LAN | 2-3 | 50-100ms | 2-5s |
| Unreliable LAN | 3-5 | 100-200ms | 5-10s |
| WAN/Internet | 5-7 | 200-500ms | 10-30s |

**Performance Considerations:**

- **More retries**: Better reliability, but higher latency on failures
- **Shorter delays**: Faster recovery, but may overwhelm failing servers
- **Exponential backoff**: Balances recovery speed with server load

**Example:**

```go
// Aggressive: Fast recovery, more retries
config := &smbfs.Config{
    RetryPolicy: &smbfs.RetryPolicy{
        MaxAttempts:  5,
        InitialDelay: 50 * time.Millisecond,
        MaxDelay:     2 * time.Second,
        Multiplier:   1.5,
    },
}

// Conservative: Slower recovery, less aggressive
config := &smbfs.Config{
    RetryPolicy: &smbfs.RetryPolicy{
        MaxAttempts:  3,
        InitialDelay: 500 * time.Millisecond,
        MaxDelay:     30 * time.Second,
        Multiplier:   3.0,
    },
}
```

---

## Best Practices

### General Recommendations

1. **Enable caching for read-heavy workloads**
   ```go
   config.Cache.EnableCache = true
   config.Cache.StatCacheTTL = 10 * time.Second
   ```

2. **Size connection pool appropriately**
   ```go
   config.MaxIdle = runtime.NumCPU()
   config.MaxOpen = runtime.NumCPU() * 2
   ```

3. **Match buffer sizes to file sizes**
   ```go
   // For large files
   config.ReadBufferSize = 256 * 1024
   config.WriteBufferSize = 256 * 1024
   ```

4. **Use timeouts appropriate for your network**
   ```go
   config.ConnTimeout = 30 * time.Second  // LAN
   config.OpTimeout = 120 * time.Second   // Large operations
   ```

### Specific Use Cases

**Web Server (Serving Static Files):**

```go
config := &smbfs.Config{
    Server:   "fileserver",
    Share:    "website",
    Username: "webuser",
    Password: "***",

    // Enable aggressive caching
    Cache: smbfs.CacheConfig{
        EnableCache:     true,
        DirCacheTTL:     60 * time.Second,
        StatCacheTTL:    60 * time.Second,
        MaxCacheEntries: 10000,
    },

    // Large pool for concurrent requests
    MaxIdle: 20,
    MaxOpen: 50,

    // Medium buffers (mixed file sizes)
    ReadBufferSize: 128 * 1024,
}
```

**Backup Application:**

```go
config := &smbfs.Config{
    Server:   "backup-server",
    Share:    "backups",
    Username: "backup",
    Password: "***",

    // Disable cache (write-heavy)
    Cache: smbfs.CacheConfig{
        EnableCache: false,
    },

    // Moderate pool
    MaxIdle: 5,
    MaxOpen: 10,

    // Large buffers for throughput
    ReadBufferSize:  512 * 1024,
    WriteBufferSize: 512 * 1024,

    // Longer timeouts for large files
    ConnTimeout: 60 * time.Second,
    OpTimeout:   300 * time.Second,
}
```

**File Indexer/Search:**

```go
config := &smbfs.Config{
    Server:   "fileserver",
    Share:    "documents",
    Username: "indexer",
    Password: "***",

    // Enable caching for metadata
    Cache: smbfs.CacheConfig{
        EnableCache:     true,
        DirCacheTTL:     30 * time.Second,
        StatCacheTTL:    30 * time.Second,
        MaxCacheEntries: 50000,
    },

    // Large pool for parallel indexing
    MaxIdle: 15,
    MaxOpen: 30,

    // Smaller buffers (metadata-focused)
    ReadBufferSize: 64 * 1024,
}
```

---

## Benchmarking

### Running Benchmarks

```bash
# Run all benchmarks
make bench

# Run specific benchmark
go test -bench=BenchmarkLargeFileRead -benchtime=10s

# Run with CPU profiling
go test -bench=. -cpuprofile=cpu.prof
go tool pprof cpu.prof
```

### Interpreting Results

```
BenchmarkSmallFileWrite-8    5000    250000 ns/op    4.00 MB/s
```

- `5000`: Number of iterations
- `250000 ns/op`: 250µs per operation
- `4.00 MB/s`: Throughput

### Custom Benchmarks

```go
func BenchmarkCustomWorkload(b *testing.B) {
    config := &smbfs.Config{
        // Your configuration
    }
    fsys, _ := smbfs.New(config)
    defer fsys.Close()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Your workload
    }
}
```

---

## Troubleshooting Performance Issues

### Slow File Operations

**Symptoms:** Individual file reads/writes are slow

**Checklist:**
1. Check network latency: `ping server`
2. Verify buffer sizes match file sizes
3. Enable debug logging to see operation times
4. Check server load and disk performance

**Solutions:**
- Increase buffer sizes for large files
- Reduce buffer sizes for small files
- Check network configuration (MTU, TCP window size)
- Verify no packet loss: `mtr server`

### High Latency for Metadata Operations

**Symptoms:** Stat() and ReadDir() are slow

**Checklist:**
1. Is caching enabled?
2. Are cache TTLs appropriate?
3. Are you reading large directories?

**Solutions:**
```go
// Enable caching
config.Cache.EnableCache = true
config.Cache.StatCacheTTL = 10 * time.Second
config.Cache.DirCacheTTL = 10 * time.Second

// Increase cache size for large directory trees
config.Cache.MaxCacheEntries = 10000
```

### Connection Pool Exhaustion

**Symptoms:** "connection pool exhausted" errors

**Checklist:**
1. Check concurrent operation count
2. Verify connections are being closed
3. Review MaxOpen setting

**Solutions:**
```go
// Increase pool size
config.MaxOpen = 50

// Ensure files are closed
defer file.Close()
```

### Memory Usage

**Symptoms:** High memory consumption

**Checklist:**
1. Check cache size (`MaxCacheEntries`)
2. Review buffer sizes
3. Verify connection pool size

**Solutions:**
```go
// Reduce cache size
config.Cache.MaxCacheEntries = 1000

// Reduce buffer sizes
config.ReadBufferSize = 64 * 1024
config.WriteBufferSize = 64 * 1024

// Reduce connection pool
config.MaxIdle = 5
config.MaxOpen = 10
```

### Inconsistent Performance

**Symptoms:** Performance varies significantly

**Possible Causes:**
- Cache misses vs hits
- Network congestion
- Server load variations
- Connection establishment overhead

**Solutions:**
- Enable logging to understand patterns
- Monitor cache hit rates
- Consider using dedicated network
- Increase cache TTLs for stable files

---

## Performance Monitoring

### Enable Logging

```go
type customLogger struct{}

func (l *customLogger) Printf(format string, v ...interface{}) {
    log.Printf("[SMBFS] "+format, v...)
}

config.Logger = &customLogger{}
```

### Metrics to Track

1. **Operation latency**: How long operations take
2. **Cache hit rate**: Percentage of cached vs uncached operations
3. **Connection pool usage**: Active vs idle connections
4. **Throughput**: Bytes/second for read/write operations
5. **Error rate**: Failed operations vs total operations

### Example Monitoring

```go
type MetricsLogger struct {
    operations int64
    cachehits  int64
    cachemiss  int64
}

func (m *MetricsLogger) LogOperation(name string, duration time.Duration) {
    atomic.AddInt64(&m.operations, 1)
    log.Printf("Operation: %s, Duration: %v", name, duration)
}

// Use with operations
start := time.Now()
info, err := fsys.Stat("/file.txt")
metrics.LogOperation("Stat", time.Since(start))
```

---

## Summary

**Quick Wins:**
1. Enable caching for read-heavy workloads
2. Size connection pool to match concurrency
3. Adjust buffer sizes based on file sizes
4. Set appropriate timeouts for your network

**Performance Formula:**

```
Throughput = (BufferSize × Concurrency) / (Latency + ProcessingTime)
```

To maximize throughput:
- Increase buffer sizes (up to network/server limits)
- Increase concurrency (MaxOpen connections)
- Reduce latency (caching, network optimization)
- Minimize processing time (efficient code)

**Remember:** Always benchmark your specific workload. Default settings work well for most cases, but tuning can provide 2-10x improvements for specialized use cases.
