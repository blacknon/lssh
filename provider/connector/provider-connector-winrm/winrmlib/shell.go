package winrmlib

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/masterzen/winrm"
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
