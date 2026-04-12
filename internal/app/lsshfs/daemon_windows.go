//go:build windows

package lsshfs

import "syscall"

func daemonSysProcAttr() *syscall.SysProcAttr {
	return nil
}
