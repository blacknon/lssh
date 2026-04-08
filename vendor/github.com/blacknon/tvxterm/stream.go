package tvxterm

import "io"

// StreamBackend adapts generic terminal-like streams to Backend.
// This is useful when another library owns session creation and only exposes
// stdin/stdout plus optional resize/close hooks.
type StreamBackend struct {
	reader io.Reader
	writer io.Writer
	resize func(cols, rows int) error
	close  func() error
}

// NewStreamBackend wraps generic terminal-like streams plus optional resize and
// close hooks into a Backend.
func NewStreamBackend(reader io.Reader, writer io.Writer, resize func(cols, rows int) error, close func() error) *StreamBackend {
	return &StreamBackend{
		reader: reader,
		writer: writer,
		resize: resize,
		close:  close,
	}
}

func (b *StreamBackend) Read(p []byte) (int, error) {
	if b.reader == nil {
		return 0, io.EOF
	}
	return b.reader.Read(p)
}

func (b *StreamBackend) Write(p []byte) (int, error) {
	if b.writer == nil {
		return 0, io.ErrClosedPipe
	}
	return b.writer.Write(p)
}

func (b *StreamBackend) Resize(cols, rows int) error {
	if b.resize == nil {
		return nil
	}
	return b.resize(cols, rows)
}

func (b *StreamBackend) Close() error {
	if b.close == nil {
		return nil
	}
	return b.close()
}
