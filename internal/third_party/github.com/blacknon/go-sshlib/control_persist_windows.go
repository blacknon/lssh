//go:build windows

package sshlib

import "os/exec"

func setDetachedSysProcAttr(cmd *exec.Cmd) {}
