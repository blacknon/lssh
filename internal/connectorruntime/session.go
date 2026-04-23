package connectorruntime

import (
	"context"
	"io"
)

type StreamConfig struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type TerminalSize struct {
	Cols int
	Rows int
}

type InteractiveSession interface {
	Run(ctx context.Context, stream StreamConfig) error
	Resize(ctx context.Context, size TerminalSize) error
	Close() error
}

type StreamSession interface {
	Run(ctx context.Context, stream StreamConfig) (int, error)
	Close() error
}
