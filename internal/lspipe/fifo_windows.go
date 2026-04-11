//go:build windows

package lspipe

import "fmt"

func createNamedPipe(path string) error {
	return fmt.Errorf("named pipes are not supported by lspipe on windows yet")
}
