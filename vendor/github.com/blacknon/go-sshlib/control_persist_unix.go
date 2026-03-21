//go:build !windows

package sshlib

import (
	"os/exec"
	"syscall"
)

func setDetachedSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
