//go:build !windows

package lsmon

import (
	"runtime"

	sshcmd "github.com/blacknon/lssh/internal/ssh"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
)

func buildRun(c *cli.Context, selected []string, controlMasterOverride *bool, shareConnect bool) *sshcmd.Run {
	run := &sshcmd.Run{
		ServerList: selected,
		RunSessionConfig: sshcmd.RunSessionConfig{
			ControlMasterOverride: controlMasterOverride,
			ShareConnect:          shareConnect,
			IsBashrc:              c.Bool("localrc"),
			IsNotBashrc:           c.Bool("not-localrc"),
		},
	}

	if runtime.GOOS != "windows" && !terminal.IsTerminal(0) {
		run.IsStdinPipe = true
	}

	return run
}
