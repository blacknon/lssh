//go:build !windows

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/blacknon/lssh/internal/connectorruntime"
	"github.com/blacknon/lssh/provider/mixed/provider-mixed-aws-ec2/ssmsession"
	"golang.org/x/crypto/ssh/terminal"
)

func runNativeSSMShell(cfg ssmsession.Config) error {
	fd := int(os.Stdin.Fd())
	session := ssmsession.New(cfg)

	if terminal.IsTerminal(fd) {
		state, err := terminal.MakeRaw(fd)
		if err != nil {
			return err
		}
		defer func() {
			_ = terminal.Restore(fd, state)
		}()

		resizeCh := make(chan os.Signal, 1)
		signal.Notify(resizeCh, syscall.SIGWINCH)
		defer signal.Stop(resizeCh)

		resizeNativeSSMSession(session, fd)
		go watchNativeSSMResize(session, fd, resizeCh)
	}

	defer session.Close()
	return session.Run(context.Background(), connectorruntime.StreamConfig{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
}

func watchNativeSSMResize(session *ssmsession.Session, fd int, resizeCh <-chan os.Signal) {
	for range resizeCh {
		resizeNativeSSMSession(session, fd)
	}
}

func resizeNativeSSMSession(session *ssmsession.Session, fd int) {
	if session == nil || !terminal.IsTerminal(fd) {
		return
	}
	cols, rows, err := terminal.GetSize(fd)
	if err != nil || cols <= 0 || rows <= 0 {
		return
	}
	_ = session.Resize(context.Background(), connectorruntime.TerminalSize{
		Cols: cols,
		Rows: rows,
	})
}
