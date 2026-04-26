//go:build darwin

package lsshfs

import (
	"golang.org/x/sys/unix"
)

func isDarwinMountActive(mountpoint string) (bool, error) {
	n, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}

	stats := make([]unix.Statfs_t, n)
	n, err = unix.Getfsstat(stats, unix.MNT_NOWAIT)
	if err != nil {
		return false, err
	}

	for _, stat := range stats[:n] {
		if unix.ByteSliceToString(stat.Mntonname[:]) == mountpoint {
			return true, nil
		}
	}
	return false, nil
}
