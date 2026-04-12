# Production Deployment Guide

Complete guide for deploying smbfs in production environments.

## Table of Contents

- [Production Checklist](#production-checklist)
- [Configuration](#configuration)
- [Security](#security)
- [Monitoring](#monitoring)
- [High Availability](#high-availability)
- [Performance Tuning](#performance-tuning)
- [Docker Deployment](#docker-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)

---

## Production Checklist

Before deploying to production:

- [ ] Review and approve [SECURITY.md](SECURITY.md) recommendations
- [ ] Configure appropriate timeouts for your network
- [ ] Set up monitoring and logging
- [ ] Test with production-like load
- [ ] Configure error handling and retries
- [ ] Set up credential management (vault/secrets)
- [ ] Review caching strategy for your workload
- [ ] Test failover scenarios
- [ ] Document operational procedures
- [ ] Set up alerts for critical errors

---

## Configuration

### Production Configuration Template

```go
package main

import (
    "log"
    "os"
    "time"
    "github.com/absfs/smbfs"
)

func NewProductionFS() (*smbfs.FileSystem, error) {
    config := &smbfs.Config{
        // Connection
        Server: os.Getenv("SMB_SERVER"),
        Port:   445,
        Share:  os.Getenv("SMB_SHARE"),

        // Authentication - USE SECRETS MANAGEMENT
        Username: os.Getenv("SMB_USERNAME"),
        Password: os.Getenv("SMB_PASSWORD"),
        Domain:   os.Getenv("SMB_DOMAIN"),

        // Connection Pool (adjust based on load)
        MaxIdle:     20,
        MaxOpen:     50,
        IdleTimeout: 5 * time.Minute,

        // Timeouts (adjust for your network)
        ConnTimeout: 30 * time.Second,
        OpTimeout:   120 * time.Second,

        // Performance (see PERFORMANCE.md)
        ReadBufferSize:  128 * 1024,  // 128 KB
        WriteBufferSize: 128 * 1024,  // 128 KB

        // Caching (enable for read-heavy workloads)
        Cache: smbfs.CacheConfig{
            EnableCache:     true,
            DirCacheTTL:     10 * time.Second,
            StatCacheTTL:    10 * time.Second,
            MaxCacheEntries: 5000,
        },

        // Reliability
        RetryPolicy: &smbfs.RetryPolicy{
            MaxAttempts:  3,
            InitialDelay: 100 * time.Millisecond,
            MaxDelay:     10 * time.Second,
            Multiplier:   2.0,
        },

        // Logging (production logger)
        Logger: &ProductionLogger{},
    }

    // Validate before creating
    if err := config.Validate(); err != nil {
        return nil, fmt.Errorf("invalid config: %w", err)
    }

    return smbfs.New(config)
}

// ProductionLogger implements structured logging
type ProductionLogger struct {
    // Your production logger (e.g., zap, logrus)
}

func (l *ProductionLogger) Printf(format string, v ...interface{}) {
    // Format as structured log
    log.Printf("[SMB] "+format, v...)
}
```

### Environment Variables

Recommended environment variables:

```bash
# Required
SMB_SERVER=fileserver.example.com
SMB_SHARE=production_data
SMB_USERNAME=service_account
SMB_PASSWORD=*** # Use secrets manager
SMB_DOMAIN=CORP

# Optional
SMB_PORT=445
SMB_MAX_CONNECTIONS=50
SMB_ENABLE_CACHE=true
SMB_CACHE_TTL=10
SMB_CONN_TIMEOUT=30
SMB_OP_TIMEOUT=120
```

Load configuration:

```go
func loadConfig() (*smbfs.Config, error) {
    config := &smbfs.Config{
        Server:   mustGetEnv("SMB_SERVER"),
        Share:    mustGetEnv("SMB_SHARE"),
        Username: mustGetEnv("SMB_USERNAME"),
        Password: mustGetEnv("SMB_PASSWORD"),
        Domain:   getEnv("SMB_DOMAIN", ""),
        Port:     getEnvInt("SMB_PORT", 445),
    }

    // Apply environment-based tuning
    if getEnvBool("SMB_ENABLE_CACHE", false) {
        config.Cache.EnableCache = true
        config.Cache.StatCacheTTL = time.Duration(getEnvInt("SMB_CACHE_TTL", 10)) * time.Second
    }

    return config, nil
}
```

---

## Security

### Credential Management

**DO NOT** hardcode credentials:

```go
// ❌ NEVER do this
config := &smbfs.Config{
    Password: "mypassword123",  // NEVER!
}

// ✅ Use environment variables
config := &smbfs.Config{
    Password: os.Getenv("SMB_PASSWORD"),
}

// ✅ Better: Use secrets manager
config := &smbfs.Config{
    Password: getSecretFromVault("smb/password"),
}
```

### Vault Integration Example

```go
import (
    vault "github.com/hashicorp/vault/api"
)

func getVaultSecret(path string) (string, error) {
    client, err := vault.NewClient(vault.DefaultConfig())
    if err != nil {
        return "", err
    }

    secret, err := client.Logical().Read(path)
    if err != nil {
        return "", err
    }

    if data, ok := secret.Data["value"].(string); ok {
        return data, nil
    }

    return "", errors.New("secret not found")
}

// Use in configuration
config := &smbfs.Config{
    Username: getVaultSecret("secret/smb/username"),
    Password: getVaultSecret("secret/smb/password"),
}
```

### TLS/Encryption

```go
// Enable SMB3 encryption (if supported by server)
config := &smbfs.Config{
    Encryption: true,  // Requires SMB 3.0+
    Signing:    true,  // Message signing
}
```

### Network Security

- Use private networks/VPNs for SMB traffic
- Never expose SMB ports (445, 139) to the Internet
- Use firewalls to restrict access
- Consider network segmentation

---

## Monitoring

### Application Metrics

Track these metrics in production:

```go
type Metrics struct {
    // Operation counters
    TotalOperations  prometheus.Counter
    FailedOperations prometheus.Counter

    // Latency histograms
    OperationDuration prometheus.Histogram

    // Connection pool
    ActiveConnections prometheus.Gauge
    PoolExhausted     prometheus.Counter

    // Cache (if implemented)
    CacheHits   prometheus.Counter
    CacheMisses prometheus.Counter
}

// Example: Track operation
func (m *Metrics) TrackOperation(name string, duration time.Duration, err error) {
    m.TotalOperations.Inc()
    if err != nil {
        m.FailedOperations.Inc()
    }
    m.OperationDuration.Observe(duration.Seconds())
}
```

### Health Checks

```go
func (fsys *FileSystem) HealthCheck() error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Try to stat root directory
    _, err := fsys.Stat("/")
    return err
}

// HTTP health endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
    if err := fsys.HealthCheck(); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        fmt.Fprintf(w, "Unhealthy: %v", err)
        return
    }
    w.WriteHeader(http.StatusOK)
    fmt.Fprint(w, "OK")
}
```

### Logging

Production logging example:

```go
type StructuredLogger struct {
    logger *zap.Logger
}

func (l *StructuredLogger) Printf(format string, v ...interface{}) {
    msg := fmt.Sprintf(format, v...)

    // Parse structured fields if possible
    l.logger.Info(msg,
        zap.String("component", "smbfs"),
        zap.Time("timestamp", time.Now()),
    )
}

// Log with context
func logOperation(ctx context.Context, operation string, err error) {
    if err != nil {
        logger.Error("SMB operation failed",
            zap.String("operation", operation),
            zap.Error(err),
            zap.String("trace_id", getTraceID(ctx)),
        )
    } else {
        logger.Debug("SMB operation succeeded",
            zap.String("operation", operation),
        )
    }
}
```

---

## High Availability

### Multiple Filesystem Instances

```go
type HAFileSystem struct {
    primaries   []*smbfs.FileSystem
    current     int
    mu          sync.RWMutex
}

func (ha *HAFileSystem) Get() *smbfs.FileSystem {
    ha.mu.RLock()
    defer ha.mu.RUnlock()
    return ha.primary[ha.current]
}

func (ha *HAFileSystem) Failover() {
    ha.mu.Lock()
    defer ha.mu.Unlock()
    ha.current = (ha.current + 1) % len(ha.primaries)
    log.Printf("Failed over to server %d", ha.current)
}

// Retry with failover
func (ha *HAFileSystem) OpenWithFailover(path string) (fs.File, error) {
    var lastErr error
    for i := 0; i < len(ha.primaries); i++ {
        file, err := ha.Get().Open(path)
        if err == nil {
            return file, nil
        }
        lastErr = err
        ha.Failover()
    }
    return nil, lastErr
}
```

### Load Balancing

```go
type LoadBalancer struct {
    filesystems []*smbfs.FileSystem
    counter     uint64
}

func (lb *LoadBalancer) Next() *smbfs.FileSystem {
    n := atomic.AddUint64(&lb.counter, 1)
    return lb.filesystems[n%uint64(len(lb.filesystems))]
}

// Round-robin load balancing
func (lb *LoadBalancer) Open(path string) (fs.File, error) {
    return lb.Next().Open(path)
}
```

---

## Performance Tuning

### Workload-Specific Configurations

**Read-Heavy (Web Server):**

```go
config := &smbfs.Config{
    Cache: smbfs.CacheConfig{
        EnableCache:     true,
        DirCacheTTL:     60 * time.Second,
        StatCacheTTL:    60 * time.Second,
        MaxCacheEntries: 10000,
    },
    MaxIdle:        20,
    MaxOpen:        50,
    ReadBufferSize: 128 * 1024,
}
```

**Write-Heavy (Backup/Upload):**

```go
config := &smbfs.Config{
    Cache: smbfs.CacheConfig{
        EnableCache: false,  // Disable for writes
    },
    MaxIdle:         10,
    MaxOpen:         20,
    WriteBufferSize: 512 * 1024,  // Large buffers
    OpTimeout:       300 * time.Second,  // Longer for large files
}
```

**Metadata-Heavy (Indexing):**

```go
config := &smbfs.Config{
    Cache: smbfs.CacheConfig{
        EnableCache:     true,
        DirCacheTTL:     30 * time.Second,
        StatCacheTTL:    30 * time.Second,
        MaxCacheEntries: 50000,  // Large cache
    },
    MaxIdle: 30,
    MaxOpen: 60,
}
```

---

## Docker Deployment

### Dockerfile

```dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /myapp ./cmd/myapp

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /myapp .

# No need to expose SMB ports (client only)
CMD ["./myapp"]
```

### Docker Compose

```yaml
version: '3.8'

services:
  myapp:
    build: .
    environment:
      - SMB_SERVER=fileserver.example.com
      - SMB_SHARE=data
      - SMB_USERNAME=${SMB_USERNAME}
      - SMB_PASSWORD=${SMB_PASSWORD}
      - SMB_DOMAIN=${SMB_DOMAIN}
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

---

## Kubernetes Deployment

### Secret Management

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: smb-credentials
type: Opaque
stringData:
  username: "serviceaccount"
  password: "***"
  domain: "CORP"
```

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 3
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      - name: myapp
        image: myapp:latest
        env:
        - name: SMB_SERVER
          value: "fileserver.example.com"
        - name: SMB_SHARE
          value: "production"
        - name: SMB_USERNAME
          valueFrom:
            secretKeyRef:
              name: smb-credentials
              key: username
        - name: SMB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: smb-credentials
              key: password
        - name: SMB_DOMAIN
          valueFrom:
            secretKeyRef:
              name: smb-credentials
              key: domain
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

---

## Operational Procedures

### Graceful Shutdown

```go
func main() {
    fsys, err := NewProductionFS()
    if err != nil {
        log.Fatal(err)
    }

    // Setup signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    // Graceful shutdown
    go func() {
        <-sigChan
        log.Println("Shutting down gracefully...")

        // Close filesystem (closes all connections)
        if err := fsys.Close(); err != nil {
            log.Printf("Error closing filesystem: %v", err)
        }

        os.Exit(0)
    }()

    // Run application
    runApp(fsys)
}
```

### Connection Lifecycle

```go
// Best practice: Single filesystem instance per application
var globalFS *smbfs.FileSystem

func init() {
    var err error
    globalFS, err = NewProductionFS()
    if err != nil {
        log.Fatal(err)
    }
}

// Reuse filesystem across requests
func handler(w http.ResponseWriter, r *http.Request) {
    file, err := globalFS.Open("/data/file.txt")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer file.Close()

    io.Copy(w, file)
}
```

---

## Production Checklist Summary

**Before Going Live:**

1. ✅ Security review completed
2. ✅ Credentials stored in secrets manager
3. ✅ Monitoring and logging configured
4. ✅ Health checks implemented
5. ✅ Resource limits set
6. ✅ Error handling tested
7. ✅ Load testing completed
8. ✅ Failover tested
9. ✅ Documentation updated
10. ✅ Runbook created

**Ongoing:**

- Monitor performance metrics
- Review logs for errors
- Test failover regularly
- Update dependencies
- Review security practices
- Optimize based on metrics

---

For more information:
- [SECURITY.md](SECURITY.md) - Security best practices
- [PERFORMANCE.md](PERFORMANCE.md) - Performance optimization
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Problem solving
