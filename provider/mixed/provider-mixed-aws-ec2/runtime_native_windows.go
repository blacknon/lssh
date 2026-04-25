//go:build windows

package main

import (
	"context"
	"os"

	"github.com/blacknon/lssh/provider/mixed/provider-mixed-aws-ec2/ssmsession"
)

func runNativeSSMShell(cfg ssmsession.Config) error {
	return ssmsession.StartShell(context.Background(), cfg, os.Stdin, os.Stdout, os.Stderr)
}
