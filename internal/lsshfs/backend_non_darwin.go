//go:build !darwin

package lsshfs

func isDarwinMountActive(mountpoint string) (bool, error) {
	return false, nil
}
