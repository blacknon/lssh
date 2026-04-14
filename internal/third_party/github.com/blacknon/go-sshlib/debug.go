package sshlib

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

const (
	debugEnv    = "GO_SSHLIB_DEBUG"
	debugLogEnv = "GO_SSHLIB_DEBUG_LOG"
)

var (
	debugLogMu   sync.Mutex
	debugLogFile *os.File
)

func debugEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(debugEnv)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func debugWriter() *os.File {
	if path := strings.TrimSpace(os.Getenv(debugLogEnv)); path != "" {
		debugLogMu.Lock()
		defer debugLogMu.Unlock()

		if debugLogFile != nil && debugLogFile.Name() == path {
			return debugLogFile
		}

		if debugLogFile != nil {
			_ = debugLogFile.Close()
			debugLogFile = nil
		}

		file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
		if err == nil {
			debugLogFile = file
			return debugLogFile
		}
	}

	return os.Stderr
}

func debugf(format string, args ...interface{}) {
	if !debugEnabled() {
		return
	}

	_, _ = fmt.Fprintf(debugWriter(), format, args...)
}

func debugln(args ...interface{}) {
	if !debugEnabled() {
		return
	}

	_, _ = fmt.Fprintln(debugWriter(), args...)
}
