//go:build windows

package lsmux

import "syscall"

func daemonSysProcAttr() *syscall.SysProcAttr {
	return nil
}
