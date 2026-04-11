// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsshfs

import (
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type Backend string

const (
	BackendFUSE Backend = "fuse"
	BackendNFS  Backend = "nfs"
	BackendSMB  Backend = "smb"
)

const defaultSMBShareName = "share"

type CommandSpec struct {
	Name string
	Args []string
}

func backendForGOOS(goos string) (Backend, error) {
	switch goos {
	case "linux":
		return BackendFUSE, nil
	case "darwin":
		return BackendNFS, nil
	case "windows":
		return BackendSMB, nil
	default:
		return "", fmt.Errorf("lsshfs does not support %s", goos)
	}
}

func runtimeBackend() (Backend, error) {
	return backendForGOOS(runtime.GOOS)
}

func normalizeMountPoint(goos, mountpoint string) (string, error) {
	mountpoint = strings.TrimSpace(mountpoint)
	if mountpoint == "" {
		return "", fmt.Errorf("mountpoint is required")
	}

	if goos == "windows" {
		mountpoint = strings.ToUpper(mountpoint)
		if !regexp.MustCompile(`^[A-Z]:$`).MatchString(mountpoint) {
			return "", fmt.Errorf("windows mountpoint must be a drive letter like Z:")
		}
		return mountpoint, nil
	}

	return filepath.Abs(mountpoint)
}

func NormalizeMountPoint(goos, mountpoint string) (string, error) {
	return normalizeMountPoint(goos, mountpoint)
}

func mountCommand(goos, mountpoint string, port int, shareName string) (CommandSpec, error) {
	switch goos {
	case "darwin":
		return CommandSpec{
			Name: "mount_nfs",
			Args: []string{
				"-o",
				fmt.Sprintf("port=%d,mountport=%d,tcp,nfsvers=3", port, port),
				"127.0.0.1:/",
				mountpoint,
			},
		}, nil
	case "windows":
		if shareName == "" {
			shareName = defaultSMBShareName
		}
		return CommandSpec{
			Name: "net",
			Args: []string{
				"use",
				mountpoint,
				fmt.Sprintf(`\\127.0.0.1\%s`, shareName),
				"/persistent:no",
			},
		}, nil
	default:
		return CommandSpec{}, fmt.Errorf("mount command is not defined for %s", goos)
	}
}

func unmountCommands(goos, mountpoint string) ([]CommandSpec, error) {
	switch goos {
	case "linux":
		return []CommandSpec{
			{Name: "fusermount3", Args: []string{"-u", mountpoint}},
			{Name: "fusermount", Args: []string{"-u", mountpoint}},
			{Name: "umount", Args: []string{mountpoint}},
		}, nil
	case "darwin":
		return []CommandSpec{
			{Name: "umount", Args: []string{mountpoint}},
		}, nil
	case "windows":
		return []CommandSpec{
			{Name: "net", Args: []string{"use", mountpoint, "/delete", "/y"}},
		}, nil
	default:
		return nil, fmt.Errorf("unmount command is not defined for %s", goos)
	}
}

func UnmountCommands(goos, mountpoint string) ([]CommandSpec, error) {
	return unmountCommands(goos, mountpoint)
}

func pickFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("failed to determine tcp port")
	}

	return addr.Port, nil
}
