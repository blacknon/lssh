//go:build !linux && !darwin

package lsshfs

import (
	"fmt"

	lsshssh "github.com/blacknon/lssh/internal/ssh"
)

func newSFTPMountConn(handle *lsshssh.SFTPClientHandle, readWrite bool) (mountConn, error) {
	return nil, fmt.Errorf("connector-backed lsshfs currently supports linux and macos only")
}
