//go:build !windows

package lsmuxsession

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"

	"golang.org/x/crypto/ssh/terminal"
)

type AttachOptions struct {
	PrefixSpec string
	DetachSpec string
}

func Attach(session Session, options AttachOptions) error {
	conn, err := DialSession(session)
	if err != nil {
		return err
	}
	defer conn.Close()

	prefixByte, err := parseAttachBindingByte(options.PrefixSpec, 0x01)
	if err != nil {
		return err
	}
	detachByte, err := parseAttachBindingByte(options.DetachSpec, 'd')
	if err != nil {
		return err
	}

	fd := int(os.Stdin.Fd())
	oldState, err := terminal.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer terminal.Restore(fd, oldState)

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(Message{Type: "attach"}); err != nil {
		return err
	}

	if cols, rows, sizeErr := terminal.GetSize(fd); sizeErr == nil {
		_ = enc.Encode(Message{Type: "resize", Cols: cols, Rows: rows})
	}

	resizeCh := make(chan os.Signal, 1)
	signal.Notify(resizeCh, syscall.SIGWINCH)
	defer signal.Stop(resizeCh)

	var encMu sync.Mutex
	send := func(msg Message) error {
		encMu.Lock()
		defer encMu.Unlock()
		return enc.Encode(msg)
	}

	go func() {
		for range resizeCh {
			if cols, rows, sizeErr := terminal.GetSize(fd); sizeErr == nil {
				_ = send(Message{Type: "resize", Cols: cols, Rows: rows})
			}
		}
	}()

	errCh := make(chan error, 2)
	go func() {
		for {
			var msg Message
			if err := dec.Decode(&msg); err != nil {
				errCh <- err
				return
			}
			switch msg.Type {
			case "output":
				if len(msg.Data) > 0 {
					_, _ = os.Stdout.Write(msg.Data)
				}
			case "exit":
				if msg.Message != "" {
					_, _ = fmt.Fprintf(os.Stderr, "\r\n%s\r\n", msg.Message)
				}
				errCh <- io.EOF
				return
			case "error":
				errCh <- fmt.Errorf("%s", msg.Message)
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, 1)
		var pendingPrefix bool
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				_ = send(Message{Type: "detach"})
				errCh <- err
				return
			}
			if n == 0 {
				continue
			}
			b := buf[0]
			if pendingPrefix {
				pendingPrefix = false
				if b == detachByte {
					_ = send(Message{Type: "detach"})
					errCh <- io.EOF
					return
				}
				if err := send(Message{Type: "input", Data: []byte{prefixByte, b}}); err != nil {
					errCh <- err
					return
				}
				continue
			}
			if b == prefixByte {
				pendingPrefix = true
				continue
			}
			if err := send(Message{Type: "input", Data: []byte{b}}); err != nil {
				errCh <- err
				return
			}
		}
	}()

	err = <-errCh
	if err == io.EOF {
		return nil
	}
	if ne, ok := err.(net.Error); ok && !ne.Timeout() {
		return nil
	}
	return err
}

func parseAttachBindingByte(spec string, fallback byte) (byte, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return fallback, nil
	}

	lower := strings.ToLower(spec)
	switch lower {
	case "tab":
		return '\t', nil
	case "enter":
		return '\r', nil
	case "esc", "escape":
		return 0x1b, nil
	}

	if strings.HasPrefix(lower, "ctrl+") && len([]rune(lower[len("ctrl+"):])) == 1 {
		r, _ := utf8.DecodeRuneInString(lower[len("ctrl+"):])
		if r >= 'a' && r <= 'z' {
			return byte(r-'a') + 1, nil
		}
		return 0, fmt.Errorf("unsupported control binding: %s", spec)
	}

	if len([]rune(spec)) == 1 {
		r, _ := utf8.DecodeRuneInString(spec)
		if r > 0x7f {
			return 0, fmt.Errorf("non-ascii binding is not supported for attach/detach: %s", spec)
		}
		return byte(r), nil
	}

	return 0, fmt.Errorf("unsupported attach/detach binding: %s", spec)
}
