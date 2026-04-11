# Contributing to smbfs

Thank you for your interest in contributing to smbfs! This document provides guidelines and information for contributors.

## Getting Started

### Prerequisites

- Go 1.21 or later
- Docker and Docker Compose (for integration tests)
- Git

### Development Setup

1. Clone the repository:
```bash
git clone https://github.com/absfs/smbfs.git
cd smbfs
```

2. Install dependencies:
```bash
go mod download
```

3. Verify the setup by running tests:
```bash
make test-quick
```

## Development Workflow

### Running Tests

```bash
# Quick tests (fastest, no race detector)
make test-quick

# Unit tests with race detector
make test-unit

# Full test suite (unit + integration)
make test-full

# Test with coverage threshold check
make test-coverage
```

### Code Quality

Before submitting a pull request:

```bash
# Format code
make fmt

# Run vet
make vet

# Run linter (requires golangci-lint)
make lint

# Run all checks
make all
```

## Writing Tests

### Unit Tests

Unit tests run without external dependencies using mock SMB backends.

**Example:**
```go
func TestFileSystem_OpenFile(t *testing.T) {
    fsys, backend, _ := setupMockFS(t)
    defer fsys.Close()

    // Add test file
    backend.AddFile("/test.txt", []byte("Hello, World!"), 0644)

    // Open and verify
    f, err := fsys.Open("/test.txt")
    if err != nil {
        t.Fatalf("Open() error = %v", err)
    }
    defer f.Close()

    content := make([]byte, 100)
    n, _ := f.Read(content)
    if string(content[:n]) != "Hello, World!" {
        t.Errorf("Read() = %q, want %q", content[:n], "Hello, World!")
    }
}
```

### Integration Tests

Integration tests require Docker and use the `integration` build tag.

**Example:**
```go
//go:build integration
// +build integration

func TestIntegration_CreateFile(t *testing.T) {
    fsys := setupTestFS(t)
    path := "/test_file.txt"

    file, err := fsys.Create(path)
    if err != nil {
        t.Fatalf("Create failed: %v", err)
    }
    defer file.Close()
    defer fsys.Remove(path)

    // ... test operations
}
```

### Test Guidelines

1. **Use `t.Helper()` in helper functions**
2. **Clean up resources** with `defer` or `t.Cleanup()`
3. **Use table-driven tests** for multiple inputs
4. **Test both success and failure cases**
5. **Run with `-race`** to detect data races
6. **Keep tests isolated** and independent

## Code Style

### Go Conventions

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable and function names
- Add comments for exported functions and types
- Keep functions focused and reasonably sized

### Error Handling

- Always check and handle errors
- Use wrapped errors for context: `fmt.Errorf("operation failed: %w", err)`
- Use standard error types where appropriate (`fs.ErrNotExist`, etc.)

### Commit Messages

Use clear, concise commit messages:

```
Add connection pool timeout handling

- Add configurable connection timeout
- Handle context cancellation in pool.get()
- Add tests for timeout scenarios
```

## Pull Request Process

1. **Create a feature branch** from `main`
2. **Write tests** for new functionality
3. **Ensure all tests pass**: `make all`
4. **Update documentation** if needed
5. **Submit a pull request** with a clear description

### PR Checklist

- [ ] Tests pass locally
- [ ] Coverage meets threshold (40%)
- [ ] Code is formatted (`make fmt`)
- [ ] No linter warnings (`make lint`)
- [ ] Documentation updated (if applicable)
- [ ] Commit messages are clear

## Architecture Overview

### Key Components

- **`filesystem.go`**: Main FileSystem implementation
- **`connection.go`**: Connection pool management
- **`file.go`**: File handle implementation
- **`smb_interfaces.go`**: Interfaces for SMB operations
- **`mock_smb.go`**: Mock implementations for testing

### Testing Architecture

```
Unit Tests (no external deps)
    ↓
MockSMBBackend ← Implements SMBShare, SMBSession, SMBFile
    ↓
ConnectionFactory → FileSystem
    ↓
Integration Tests (with Docker)
    ↓
Real SMB Server
```

## Reporting Issues

When reporting issues:

1. **Check existing issues** first
2. **Provide minimal reproduction** steps
3. **Include environment details** (Go version, OS, etc.)
4. **Attach relevant logs** or error messages

## Questions?

- Open a GitHub issue for questions
- Check existing documentation in `TESTING.md`
- Review existing tests for examples

Thank you for contributing!
