//go:build windows

package lssh

import "syscall"

func daemonSysProcAttr() *syscall.SysProcAttr {
	return nil
}
