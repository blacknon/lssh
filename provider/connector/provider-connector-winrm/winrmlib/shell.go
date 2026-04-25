package winrmlib

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"

	"github.com/masterzen/winrm"
	"golang.org/x/crypto/ssh/terminal"
)

var ErrUnsupported = errors.New("unsupported")

type TerminalOptions struct {
	Term       string
	Cols       int
	Rows       int
	StartShell bool
	Command    string
}

type Terminal struct {
	command *winrm.Command
	shell   *winrm.Shell
	cancel  context.CancelFunc
}

type Command struct {
	command *winrm.Command
	shell   *winrm.Shell
	cancel  context.CancelFunc
}

func (c *Connect) OpenTerminal(opts TerminalOptions) (*Terminal, error) {
	if !opts.StartShell {
		return nil, fmt.Errorf("winrmlib: terminal requires StartShell")
	}

	shell, err := c.client.CreateShell()
	if err != nil {
		return nil, fmt.Errorf("create winrm shell: %w", err)
	}

	commandLine := c.cfg.ShellCommand
	if commandLine == "" {
		commandLine = "cmd.exe"
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd, err := shell.ExecuteWithContext(ctx, commandLine)
	if err != nil {
		cancel()
		_ = shell.Close()
		return nil, fmt.Errorf("execute shell command %q: %w", commandLine, err)
	}

	return &Terminal{command: cmd, shell: shell, cancel: cancel}, nil
}

func (c *Connect) OpenShell() (*Terminal, error) {
	return c.OpenTerminal(TerminalOptions{StartShell: true})
}

func (t *Terminal) Stdin() io.Writer  { return t.command.Stdin }
func (t *Terminal) Stdout() io.Reader { return t.command.Stdout }
func (t *Terminal) Stderr() io.Reader { return t.command.Stderr }
func (t *Terminal) Resize(cols, rows int) error {
	return ErrUnsupported
}

func (t *Terminal) Wait() error {
	t.command.Wait()
	return t.command.Error()
}

func (t *Terminal) Interact(stdin *os.File, stdout, stderr *os.File) error {
	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	restoreInput, err := prepareInteractiveInput(stdin)
	if err != nil {
		return err
	}
	defer restoreInput()

	var wg sync.WaitGroup
	if out := t.Stdout(); out != nil && stdout != nil {
		wg.Add(1)
		go copySessionStream(&wg, stdout, out)
	}
	if errOut := t.Stderr(); errOut != nil && stderr != nil {
		wg.Add(1)
		go copySessionStream(&wg, stderr, errOut)
	}

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-interrupts:
				_, _ = t.command.Stdin.Write([]byte{3})
			}
		}
	}()

	inputErrCh := make(chan error, 1)
	go func() {
		inputErrCh <- copyInteractiveInput(t.command.Stdin, stdin, stdout)
	}()

	waitErr := t.Wait()
	wg.Wait()

	select {
	case inputErr := <-inputErrCh:
		if inputErr != nil && !errors.Is(inputErr, io.EOF) {
			return inputErr
		}
	default:
	}

	return waitErr
}

func (t *Terminal) Close() error {
	return closeWinRMSession(t.command, t.shell, t.cancel)
}

func (c *Connect) ExecuteCommand(commandLine string) (*Command, error) {
	shell, err := c.client.CreateShell()
	if err != nil {
		return nil, fmt.Errorf("create winrm shell: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd, err := shell.ExecuteWithContext(ctx, commandLine)
	if err != nil {
		cancel()
		_ = shell.Close()
		return nil, fmt.Errorf("execute command %q: %w", commandLine, err)
	}

	return &Command{command: cmd, shell: shell, cancel: cancel}, nil
}

func (c *Command) Stdin() io.Writer  { return c.command.Stdin }
func (c *Command) Stdout() io.Reader { return c.command.Stdout }
func (c *Command) Stderr() io.Reader { return c.command.Stderr }

func (c *Command) Wait() error {
	c.command.Wait()
	return c.command.Error()
}

func (c *Command) ExitCode() int {
	if c == nil || c.command == nil {
		return 0
	}
	return c.command.ExitCode()
}

func (c *Command) Close() error {
	return closeWinRMSession(c.command, c.shell, c.cancel)
}

func closeWinRMSession(cmd *winrm.Command, shell *winrm.Shell, cancel context.CancelFunc) error {
	if cancel != nil {
		cancel()
	}

	var firstErr error
	if cmd != nil {
		if err := cmd.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if shell != nil {
		if err := shell.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func copySessionStream(wg *sync.WaitGroup, dst io.Writer, src io.Reader) {
	defer wg.Done()
	_, _ = io.Copy(dst, src)
}

func copyInteractiveInput(dst io.Writer, src *os.File, localOut io.Writer) error {
	if src == nil {
		return nil
	}
	if !terminal.IsTerminal(int(src.Fd())) {
		_, err := io.Copy(dst, src)
		return err
	}

	reader := bufio.NewReader(src)
	var lineBuf []byte
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		switch b {
		case '\r', '\n':
			command := string(lineBuf)
			if shouldClearLocally(command) {
				clearScreen(localOut)
			} else if localOut != nil {
				_, _ = localOut.Write([]byte("\r\n"))
			}
			if _, writeErr := io.WriteString(dst, "\r\n"); writeErr != nil {
				return writeErr
			}
			lineBuf = lineBuf[:0]
		case 3:
			lineBuf = lineBuf[:0]
			if _, writeErr := dst.Write([]byte{3}); writeErr != nil {
				return writeErr
			}
			if localOut != nil {
				_, _ = localOut.Write([]byte("^C\r\n"))
			}
		case 9:
			if _, writeErr := dst.Write([]byte{b}); writeErr != nil {
				return writeErr
			}
		case 12:
			clearScreen(localOut)
		case 8, 127:
			if len(lineBuf) == 0 {
				continue
			}
			lineBuf = lineBuf[:len(lineBuf)-1]
			if _, writeErr := dst.Write([]byte{8}); writeErr != nil {
				return writeErr
			}
			if localOut != nil {
				_, _ = localOut.Write([]byte("\b \b"))
			}
		default:
			if b < 32 {
				if _, writeErr := dst.Write([]byte{b}); writeErr != nil {
					return writeErr
				}
				continue
			}
			lineBuf = append(lineBuf, b)
			if _, writeErr := dst.Write([]byte{b}); writeErr != nil {
				return writeErr
			}
			if localOut != nil {
				_, _ = localOut.Write([]byte{b})
			}
		}
	}
}

func shouldClearLocally(command string) bool {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "clear", "cls", "clear-host":
		return true
	default:
		return false
	}
}

func clearScreen(w io.Writer) {
	if w == nil {
		return
	}
	_, _ = io.WriteString(w, "\033[2J\033[H")
}

func prepareInteractiveInput(stdin *os.File) (func(), error) {
	if stdin == nil || !terminal.IsTerminal(int(stdin.Fd())) {
		return func() {}, nil
	}

	state, err := terminal.MakeRaw(int(stdin.Fd()))
	if err != nil {
		return nil, err
	}

	return func() {
		_ = terminal.Restore(int(stdin.Fd()), state)
	}, nil
}
