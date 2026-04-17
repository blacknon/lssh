package telnetlib

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const (
	iac  = 255
	dont = 254
	doo  = 253
	wont = 252
	will = 251
	sb   = 250
	se   = 240
)

func (c *Connect) bootstrapLogin() error {
	reader := newDataReader(c.conn)
	writer := &escapedWriter{w: c.conn}

	if c.cfg.InitialNewline {
		if _, err := writer.Write([]byte("\n")); err != nil {
			return fmt.Errorf("send initial newline: %w", err)
		}
	}

	if strings.TrimSpace(c.cfg.Username) != "" {
		if _, err := readUntilPrompt(reader, c.cfg.LoginPrompt, 5*time.Second); err != nil {
			return fmt.Errorf("wait login prompt: %w", err)
		}
		if _, err := writer.Write([]byte(c.cfg.Username + "\n")); err != nil {
			return fmt.Errorf("send username: %w", err)
		}
	}

	if strings.TrimSpace(c.cfg.Password) != "" {
		if _, err := readUntilPrompt(reader, c.cfg.PasswordPrompt, 5*time.Second); err != nil {
			return fmt.Errorf("wait password prompt: %w", err)
		}
		if _, err := writer.Write([]byte(c.cfg.Password + "\n")); err != nil {
			return fmt.Errorf("send password: %w", err)
		}
	}

	if c.cfg.PostLoginDelayMs > 0 {
		time.Sleep(time.Duration(c.cfg.PostLoginDelayMs) * time.Millisecond)
	}
	return nil
}

type escapedWriter struct {
	w  io.Writer
	mu sync.Mutex
}

func (w *escapedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	out := make([]byte, 0, len(p)+8)
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case '\r':
			out = appendTelnetByte(out, '\r')
			out = appendTelnetByte(out, '\n')
			if i+1 < len(p) && p[i+1] == '\n' {
				i++
			}
		case '\n':
			out = appendTelnetByte(out, '\r')
			out = appendTelnetByte(out, '\n')
		default:
			out = appendTelnetByte(out, p[i])
		}
	}
	if _, err := w.w.Write(out); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *escapedWriter) Close() error {
	return nil
}

func appendTelnetByte(dst []byte, b byte) []byte {
	dst = append(dst, b)
	if b == iac {
		dst = append(dst, iac)
	}
	return dst
}

type dataReader struct {
	r   io.Reader
	buf []byte
	mu  sync.Mutex
}

func newDataReader(r io.Reader) *dataReader {
	return &dataReader{r: r, buf: make([]byte, 0, 4096)}
}

func (r *dataReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for len(r.buf) == 0 {
		tmp := make([]byte, 1024)
		n, err := r.r.Read(tmp)
		if n > 0 {
			r.buf = append(r.buf, filterTelnetCommands(tmp[:n])...)
		}
		if len(r.buf) > 0 {
			break
		}
		if err != nil {
			return 0, err
		}
	}

	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

func filterTelnetCommands(data []byte) []byte {
	out := make([]byte, 0, len(data))
	for i := 0; i < len(data); i++ {
		if data[i] != iac {
			out = append(out, data[i])
			continue
		}
		if i+1 >= len(data) {
			break
		}
		next := data[i+1]
		switch next {
		case iac:
			out = append(out, iac)
			i++
		case will, wont, doo, dont:
			i += 2
		case sb:
			i += 2
			for i < len(data)-1 {
				if data[i] == iac && data[i+1] == se {
					i++
					break
				}
				i++
			}
		default:
			i++
		}
	}
	return out
}

func readUntilPrompt(r io.Reader, prompt string, timeout time.Duration) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		return "", nil
	}

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	type result struct {
		text string
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		buf := make([]byte, 256)
		var acc bytes.Buffer
		promptLower := strings.ToLower(prompt)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				acc.Write(buf[:n])
				if strings.Contains(strings.ToLower(acc.String()), promptLower) {
					ch <- result{text: acc.String(), err: nil}
					return
				}
			}
			if err != nil {
				ch <- result{text: acc.String(), err: err}
				return
			}
		}
	}()

	select {
	case res := <-ch:
		return res.text, res.err
	case <-deadline.C:
		return "", fmt.Errorf("timeout waiting for prompt %q", prompt)
	}
}
