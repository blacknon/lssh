package telnetlib

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
)

var ErrUnsupported = errors.New("unsupported")

type TerminalOptions struct {
	Term       string
	Cols       int
	Rows       int
	StartShell bool
}

type Terminal struct {
	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader

	conn      io.Closer
	waitCh    chan error
	closeOnce sync.Once
}

func (c *Connect) OpenTerminal(opts TerminalOptions) (*Terminal, error) {
	if !opts.StartShell {
		return nil, errors.New("telnetlib: terminal requires StartShell")
	}

	if err := c.bootstrapLogin(); err != nil {
		_ = c.Close()
		return nil, err
	}

	writer := &escapedWriter{w: c.conn}
	reader := newDataReader(c.conn)
	stdoutReader, stdoutWriter := io.Pipe()

	t := &Terminal{
		stdin:  writer,
		stdout: stdoutReader,
		stderr: bytes.NewReader(nil),
		conn:   c.conn,
		waitCh: make(chan error, 1),
	}

	go t.copyOutput(reader, stdoutWriter)
	return t, nil
}

func (c *Connect) OpenShell() (*Terminal, error) {
	return c.OpenTerminal(TerminalOptions{StartShell: true})
}

func OpenShell(cfg TargetConfig, term string, cols, rows int) (*Terminal, error) {
	con, err := Dial(cfg)
	if err != nil {
		return nil, err
	}

	return openShellWithConnect(con, term, cols, rows)
}

func OpenShellWithConn(cfg TargetConfig, conn net.Conn, term string, cols, rows int) (*Terminal, error) {
	con := &Connect{
		cfg:  cfg,
		conn: conn,
	}
	return openShellWithConnect(con, term, cols, rows)
}

func openShellWithConnect(con *Connect, term string, cols, rows int) (*Terminal, error) {
	sess, err := con.OpenTerminal(TerminalOptions{
		Term:       term,
		Cols:       cols,
		Rows:       rows,
		StartShell: true,
	})
	if err != nil {
		_ = con.Close()
		return nil, err
	}
	return sess, nil
}

func (t *Terminal) copyOutput(reader io.Reader, stdoutWriter *io.PipeWriter) {
	defer stdoutWriter.Close()

	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if _, writeErr := stdoutWriter.Write(buf[:n]); writeErr != nil {
				t.waitCh <- writeErr
				return
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				t.waitCh <- nil
			} else {
				t.waitCh <- err
			}
			return
		}
	}
}

func (t *Terminal) Resize(cols, rows int) error {
	return ErrUnsupported
}

func (t *Terminal) Stdin() io.Writer {
	return t.stdin
}

func (t *Terminal) Stdout() io.Reader {
	return t.stdout
}

func (t *Terminal) Stderr() io.Reader {
	return t.stderr
}

func (t *Terminal) Wait() error {
	if t == nil {
		return errors.New("telnetlib: terminal is nil")
	}
	return <-t.waitCh
}

func (t *Terminal) Close() error {
	if t == nil {
		return nil
	}

	var err error
	t.closeOnce.Do(func() {
		if t.stdin != nil {
			_ = t.stdin.Close()
		}
		if t.conn != nil {
			err = t.conn.Close()
		}
	})
	return err
}
