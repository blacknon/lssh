//go:build windows

package ssh

import (
	"context"
	"io"

	"github.com/blacknon/lssh/internal/connectorruntime"
	"github.com/blacknon/lssh/provider/mixed/provider-mixed-aws-ec2/ssmsession"
)

func runNativeSSMShell(cfg ssmsession.Config, stdin io.Reader, stdout, stderr io.Writer) error {
	session := ssmsession.New(cfg)
	defer session.Close()

	return runNativeInteractiveSession(func() error {
		return session.Run(context.Background(), connectorruntime.StreamConfig{
			Stdin:  stdin,
			Stdout: stdout,
			Stderr: stderr,
		})
	})
}
