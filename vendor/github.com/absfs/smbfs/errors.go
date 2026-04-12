package smbfs

import (
	"errors"
	"io/fs"
)

var (
	// ErrNotImplemented indicates a feature is not yet implemented.
	ErrNotImplemented = errors.New("not implemented")

	// ErrInvalidConfig indicates the configuration is invalid.
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrConnectionClosed indicates the connection has been closed.
	ErrConnectionClosed = errors.New("connection closed")

	// ErrPoolExhausted indicates all connections in the pool are in use.
	ErrPoolExhausted = errors.New("connection pool exhausted")

	// ErrAuthenticationFailed indicates authentication failed.
	ErrAuthenticationFailed = errors.New("authentication failed")

	// ErrUnsupportedDialect indicates the SMB dialect is not supported.
	ErrUnsupportedDialect = errors.New("unsupported SMB dialect")

	// ErrInvalidPath indicates the path is invalid.
	ErrInvalidPath = errors.New("invalid path")

	// ErrNotDirectory indicates the path is not a directory.
	ErrNotDirectory = errors.New("not a directory")

	// ErrIsDirectory indicates the path is a directory.
	ErrIsDirectory = errors.New("is a directory")
)

// wrapPathError wraps an error with operation and path information.
// Uses fs.PathError to ensure compatibility with os.IsNotExist and other stdlib checks.
func wrapPathError(op, path string, err error) error {
	if err == nil {
		return nil
	}

	// If it's already a PathError for the same path, don't double-wrap
	var pe *fs.PathError
	if errors.As(err, &pe) && pe.Path == path {
		return err
	}

	return &fs.PathError{
		Op:   op,
		Path: path,
		Err:  err,
	}
}

// convertError converts common errors to fs package errors.
func convertError(err error) error {
	if err == nil {
		return nil
	}

	// Already a standard error
	if errors.Is(err, fs.ErrNotExist) ||
		errors.Is(err, fs.ErrExist) ||
		errors.Is(err, fs.ErrPermission) ||
		errors.Is(err, fs.ErrInvalid) ||
		errors.Is(err, fs.ErrClosed) {
		return err
	}

	// Map our errors to standard fs errors
	switch {
	case errors.Is(err, ErrConnectionClosed):
		return fs.ErrClosed
	case errors.Is(err, ErrInvalidPath):
		return fs.ErrInvalid
	case errors.Is(err, ErrAuthenticationFailed):
		return fs.ErrPermission
	}

	return err
}

// netError interface for network errors.
type netError interface {
	Timeout() bool
	Temporary() bool
}

// isRetryable returns true if the error indicates a transient failure
// that might succeed if retried.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Network errors are generally retryable
	var netErr netError
	if errors.As(err, &netErr) {
		// Temporary network errors are retryable
		if netErr.Temporary() {
			return true
		}
		// Timeout errors are retryable
		if netErr.Timeout() {
			return true
		}
	}

	// Connection errors are typically retryable
	switch {
	case errors.Is(err, ErrConnectionClosed):
		return true
	case errors.Is(err, ErrPoolExhausted):
		return true
	}

	// Check wrapped errors
	unwrapped := errors.Unwrap(err)
	if unwrapped != nil && unwrapped != err {
		return isRetryable(unwrapped)
	}

	return false
}
