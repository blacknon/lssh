package sshlib

import (
	"fmt"
	"os"
)

type remotePathClient interface {
	RealPath(string) (string, error)
	Stat(string) (os.FileInfo, error)
	ReadDir(string) ([]os.FileInfo, error)
}

func resolveRemoteBasepoint(client remotePathClient, path string) (string, error) {
	homepoint, err := client.RealPath(".")
	if err != nil {
		return "", err
	}

	basepoint := getRemoteAbsPath(homepoint, path)
	info, err := client.Stat(basepoint)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("remote path is not a directory: %s", basepoint)
	}
	if _, err := client.ReadDir(basepoint); err != nil {
		return "", err
	}

	return basepoint, nil
}
