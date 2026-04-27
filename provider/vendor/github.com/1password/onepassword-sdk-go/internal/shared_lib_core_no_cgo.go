//go:build !cgo && (darwin || linux)

package internal

import "fmt"

func GetSharedLibCore(accountName string) (*CoreWrapper, error) {
	return nil, fmt.Errorf("You've probably hit this during cross-compilation: The desktop app integration feature requires CGO (CGO_ENABLED=1 and a working C toolchain - see README.md for how to build this)")
}
