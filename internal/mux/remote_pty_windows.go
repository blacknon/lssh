//go:build windows

package mux

import "os"

func configureNativeSessionPTY(tty *os.File) error {
	return nil
}
