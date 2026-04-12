# Testing Guide

This document describes the testing infrastructure for smbfs.

## Test Types

### 1. Unit Tests

Unit tests run without external dependencies and test individual components in isolation.

**Run unit tests:**
```bash
make test-unit
# or
go test -v -race -count=1 ./...
```

**Coverage:**
- 38 unit tests covering:
  - Configuration validation
  - Path normalization
  - Error handling
  - Retry logic
  - Connection pool logic (without real connections)

### 2. Integration Tests

Integration tests run against a real Samba server using Docker.

**Prerequisites:**
- Docker and Docker Compose installed
- Port 445 available on localhost

**Run integration tests:**
```bash
make test-integration
```

This will:
1. Start a Docker Samba server
2. Run integration tests
3. Stop the Docker server

**Manual control:**
```bash
# Start server
make docker-up

# Run tests manually
SMB_SERVER=localhost \
SMB_SHARE=testshare \
SMB_USERNAME=testuser \
SMB_PASSWORD=testpass123 \
SMB_DOMAIN=TESTGROUP \
go test -v -tags=integration ./...

# Stop server
make docker-down
```

**Integration test coverage:**
- Basic connection and authentication
- File create, read, write, delete
- Directory operations (mkdir, readdir, remove)
- Nested directory creation (MkdirAll)
- File rename
- File time modification (Chtimes)
- File permission changes (Chmod)
- Large file handling (10MB)
- Concurrent operations

### 3. Benchmarks

Performance benchmarks measure throughput and latency.

**Run benchmarks:**
```bash
make bench
# or
go test -bench=. -benchmem -tags=integration ./...
```

**Benchmarks:**
- File creation
- Small/medium/large file writes (1KB, 64KB, 1MB)
- Small/medium/large file reads
- Stat operations
- Directory reading
- Directory creation
- File rename
- Chmod/Chtimes operations
- Sequential reads
- Connection pool efficiency

**Example output:**
```
BenchmarkSmallFileWrite-8     100    11234567 ns/op    0.09 MB/s    1024 B/op    15 allocs/op
BenchmarkMediumFileWrite-8     50    23456789 ns/op    2.81 MB/s   65536 B/op    18 allocs/op
BenchmarkLargeFileWrite-8      10   123456789 ns/op    8.54 MB/s 1048576 B/op    21 allocs/op
```

## Docker Test Environment

### Samba Server Configuration

The Docker Compose setup provides:
- **Service:** dperson/samba
- **Shares:**
  - `testshare` - Authenticated share (testuser)
  - `public` - Public share (guest access)
- **Credentials:**
  - Username: `testuser`
  - Password: `testpass123`
  - Domain: `TESTGROUP`
- **Port:** 445 (mapped to localhost)

### Docker Commands

```bash
# Start server
docker-compose up -d

# View logs
make docker-logs
# or
docker-compose logs -f samba

# Restart server
make docker-restart

# Stop server
docker-compose down

# Clean up (including volumes)
make clean
```

### Troubleshooting

**Port 445 already in use:**
```bash
# Check what's using port 445
sudo lsof -i :445
# or
sudo netstat -tlnp | grep :445

# On macOS, you may need to disable built-in SMB:
sudo launchctl unload -w /System/Library/LaunchDaemons/com.apple.smbd.plist
```

**Cannot connect to Docker Samba:**
```bash
# Check if container is running
docker ps | grep smbfs-test-server

# Check container logs
docker logs smbfs-test-server

# Test connectivity
smbclient -L //localhost/testshare -U testuser%testpass123
```

**Tests fail intermittently:**
- Increase sleep time in Makefile (default: 5s)
- Check Docker resources (CPU, memory)
- Run tests with `-count=1` to disable caching

## Code Coverage

Generate HTML coverage report:

```bash
make coverage
```

This creates `coverage.html` that you can open in a browser.

**Target:** Maintain >80% code coverage for critical paths.

## Continuous Integration

### GitHub Actions (Recommended)

Create `.github/workflows/test.yml`:

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v3

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.23'

    - name: Run unit tests
      run: make test-unit

    - name: Run integration tests
      run: make test-integration

    - name: Generate coverage
      run: make coverage

    - name: Upload coverage
      uses: codecov/codecov-action@v3
      with:
        file: ./coverage.out
```

## Test Organization

```
smbfs/
├── *_test.go           # Unit tests (no build tags)
├── integration_test.go # Integration tests (build tag: integration)
├── benchmark_test.go   # Benchmarks (build tag: integration)
├── docker-compose.yml  # Test Samba server
├── Makefile           # Test automation
└── TESTING.md         # This file
```

## Writing Tests

### Unit Test Example

```go
func TestPathNormalizer(t *testing.T) {
    pn := newPathNormalizer(false)

    tests := []struct {
        input    string
        expected string
    }{
        {"/path/to/file", "/path/to/file"},
        {"\\path\\to\\file", "/path/to/file"},
    }

    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            result := pn.normalize(tt.input)
            if result != tt.expected {
                t.Errorf("normalize(%q) = %q, want %q",
                    tt.input, result, tt.expected)
            }
        })
    }
}
```

### Integration Test Example

```go
//go:build integration
// +build integration

func TestIntegration_CreateFile(t *testing.T) {
    fsys := setupTestFS(t)  // Helper function

    path := "/test_file.txt"
    content := []byte("test content")

    // Create
    file, err := fsys.Create(path)
    if err != nil {
        t.Fatalf("Create failed: %v", err)
    }

    // Write
    file.Write(content)
    file.Close()

    // Verify
    data, err := fs.ReadFile(fsys, path)
    if err != nil {
        t.Fatalf("ReadFile failed: %v", err)
    }

    if !bytes.Equal(data, content) {
        t.Errorf("Content mismatch")
    }

    // Cleanup
    fsys.Remove(path)
}
```

### Benchmark Example

```go
//go:build integration
// +build integration

func BenchmarkFileWrite(b *testing.B) {
    fsys := setupTestFS(b.(*testing.T))
    data := bytes.Repeat([]byte("x"), 1024) // 1KB

    b.ResetTimer()
    b.SetBytes(int64(len(data)))

    for i := 0; i < b.N; i++ {
        path := fmt.Sprintf("/bench_%d.txt", i)
        file, _ := fsys.Create(path)
        file.Write(data)
        file.Close()
        fsys.Remove(path)
    }
}
```

## Best Practices

1. **Always use `t.Helper()` in test helper functions**
2. **Clean up resources in `defer` or `t.Cleanup()`**
3. **Use `t.Run()` for subtests**
4. **Test both success and failure cases**
5. **Use table-driven tests for multiple inputs**
6. **Run with `-race` to detect race conditions**
7. **Set `-count=1` to disable test caching**
8. **Use meaningful test names**
9. **Keep tests isolated and independent**
10. **Mock external dependencies in unit tests**

## Performance Testing

### Baseline Benchmarks

Run benchmarks and save results:

```bash
go test -bench=. -benchmem -tags=integration ./... | tee baseline.txt
```

After making changes:

```bash
go test -bench=. -benchmem -tags=integration ./... | tee new.txt
benchstat baseline.txt new.txt
```

### Profiling

**CPU Profile:**
```bash
go test -cpuprofile=cpu.prof -bench=BenchmarkLargeFileWrite -tags=integration
go tool pprof cpu.prof
```

**Memory Profile:**
```bash
go test -memprofile=mem.prof -bench=BenchmarkLargeFileWrite -tags=integration
go tool pprof mem.prof
```

## Test Environments

### Local Development
```bash
make test-unit          # Fast, no dependencies
make test-integration   # Full integration tests
```

### CI/CD
```bash
make all               # Format, vet, lint, test
```

### Pre-commit
```bash
make fmt vet test-unit
```

### Pre-release
```bash
make all
make test-integration
make bench
make coverage
```

## Debugging Tests

**Verbose output:**
```bash
go test -v ./...
```

**Run specific test:**
```bash
go test -v -run TestIntegration_CreateFile ./...
```

**Run with race detector:**
```bash
go test -race ./...
```

**Increase verbosity:**
```bash
go test -v -race -count=1 -tags=integration ./... 2>&1 | tee test.log
```

## Known Issues

1. **Port 445 conflicts**: Some systems have SMB services on 445
2. **Docker on macOS**: May need specific Docker Desktop settings
3. **Windows Firewall**: May block port 445
4. **Network latency**: Affects integration test timing

## Contributing

When contributing:

1. Add unit tests for new functionality
2. Add integration tests for user-facing features
3. Run `make all` before submitting PR
4. Ensure tests pass with `-race` flag
5. Update this document if adding new test infrastructure

## Questions?

- Check existing tests for examples
- See `Makefile` for available commands
- Refer to Go testing documentation: https://pkg.go.dev/testing
