//go:build !windows

package lspipe

import "golang.org/x/sys/unix"

func createNamedPipe(path string) error {
	return unix.Mkfifo(path, 0o600)
}
