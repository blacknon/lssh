package tvxterm

import (
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// Backend is the transport abstraction for the terminal widget.
// Anything that can read terminal output, accept terminal input, and resize
// a terminal session can be attached.
type Backend interface {
	io.Reader
	io.Writer
	Resize(cols, rows int) error
	Close() error
}

// PTYBackend wraps a local PTY-backed process.
type PTYBackend struct {
	ptmx *os.File
	cmd  *exec.Cmd
	once sync.Once
}

// NewPTYBackend starts cmd under a PTY and returns a Backend implementation for
// it with the requested initial size.
func NewPTYBackend(cmd *exec.Cmd, cols, rows int) (*PTYBackend, error) {
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}
	return &PTYBackend{ptmx: f, cmd: cmd}, nil
}

func (b *PTYBackend) Read(p []byte) (int, error)  { return b.ptmx.Read(p) }
func (b *PTYBackend) Write(p []byte) (int, error) { return b.ptmx.Write(p) }

func (b *PTYBackend) Resize(cols, rows int) error {
	return pty.Setsize(b.ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

func (b *PTYBackend) Close() error {
	var err error
	b.once.Do(func() {
		if b.cmd != nil && b.cmd.Process != nil {
			_ = b.cmd.Process.Kill()
			_, _ = b.cmd.Process.Wait()
		}
		err = b.ptmx.Close()
	})
	return err
}
