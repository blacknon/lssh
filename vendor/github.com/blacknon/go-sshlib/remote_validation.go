package sshlib

import (
	"fmt"

	"github.com/pkg/sftp"
)

func resolveAndValidateRemoteDir(client *sftp.Client, basepoint string) (string, error) {
	homepoint, err := client.RealPath(".")
	if err != nil {
		return "", err
	}

	absBasepoint := getRemoteAbsPath(homepoint, basepoint)
	info, err := client.Stat(absBasepoint)
	if err != nil {
		return "", fmt.Errorf("sshlib: remote path validation failed for %s: %w", absBasepoint, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("sshlib: remote path is not a directory: %s", absBasepoint)
	}
	if _, err := client.ReadDir(absBasepoint); err != nil {
		return "", fmt.Errorf("sshlib: remote path is not readable: %s: %w", absBasepoint, err)
	}

	return absBasepoint, nil
}
