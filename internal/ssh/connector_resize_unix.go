//go:build !windows

package ssh

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/blacknon/lssh/internal/connectorruntime"
	"github.com/blacknon/lssh/provider/mixed/provider-mixed-aws-ec2/ssmsession"
	"golang.org/x/crypto/ssh/terminal"
)

func runNativeSSMShell(cfg ssmsession.Config, stdin io.Reader, stdout, stderr io.Writer) error {
	session := ssmsession.New(cfg)
	defer session.Close()

	return runNativeInteractiveSession(func() error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			errCh <- session.Run(ctx, connectorruntime.StreamConfig{
				Stdin:  stdin,
				Stdout: stdout,
				Stderr: stderr,
			})
		}()

		fd := int(os.Stdin.Fd())
		if terminal.IsTerminal(fd) {
			resizeCh := make(chan os.Signal, 1)
			signal.Notify(resizeCh, syscall.SIGWINCH)
			defer signal.Stop(resizeCh)

			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-resizeCh:
						cols, rows, err := terminal.GetSize(fd)
						if err != nil {
							continue
						}
						_ = session.Resize(ctx, connectorruntime.TerminalSize{
							Cols: cols,
							Rows: rows,
						})
					}
				}
			}()
		}

		return <-errCh
	})
}
