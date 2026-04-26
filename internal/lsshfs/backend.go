// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsshfs

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type Backend string

const (
	BackendFUSE Backend = "fuse"
	BackendNFS  Backend = "nfs"
)

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
		return "", fmt.Errorf("lsshfs does not support windows in 0.10.0")
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

	absMountpoint, err := filepath.Abs(mountpoint)
	if err != nil {
		return "", err
	}

	return resolveMountPath(absMountpoint), nil
}

func NormalizeMountPoint(goos, mountpoint string) (string, error) {
	return normalizeMountPoint(goos, mountpoint)
}

func mountCommand(goos, mountpoint string, port int, shareName string, creds interface{}, mountOptions []string) (CommandSpec, error) {
	switch goos {
	case "linux":
		options := formatMountOptions(
			[]string{fmt.Sprintf("port=%d", port), fmt.Sprintf("mountport=%d", port), "tcp", "nfsvers=3"},
			mountOptions,
		)
		return CommandSpec{
			Name: "mount",
			Args: []string{
				"-t",
				"nfs",
				"-o",
				options,
				"127.0.0.1:/",
				mountpoint,
			},
		}, nil
	case "darwin":
		options := formatMountOptions(
			[]string{fmt.Sprintf("port=%d", port), fmt.Sprintf("mountport=%d", port), "tcp", "nfsvers=3"},
			mountOptions,
		)
		return CommandSpec{
			Name: "mount_nfs",
			Args: []string{
				"-o",
				options,
				"127.0.0.1:/",
				mountpoint,
			},
		}, nil
	case "windows":
		return CommandSpec{
			Name: "mount",
			Args: []string{
				"-o",
				"anon,mtype=hard",
				"127.0.0.1:/",
				mountpoint,
			},
		}, nil
	default:
		return CommandSpec{}, fmt.Errorf("mount command is not defined for %s", goos)
	}
}

func formatMountOptions(base []string, extra []string) string {
	seen := map[string]struct{}{}
	options := make([]string, 0, len(base)+len(extra))
	for _, group := range [][]string{base, normalizeMountOptions(extra)} {
		for _, option := range group {
			option = strings.TrimSpace(option)
			if option == "" {
				continue
			}
			if _, ok := seen[option]; ok {
				continue
			}
			seen[option] = struct{}{}
			options = append(options, option)
		}
	}

	return strings.Join(options, ",")
}

func normalizeMountOptions(options []string) []string {
	result := make([]string, 0, len(options))
	for _, option := range options {
		for _, part := range strings.Split(option, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

func NormalizeMountOptions(options []string) []string {
	return normalizeMountOptions(options)
}

func unmountCommands(goos, mountpoint string) ([]CommandSpec, error) {
	switch goos {
	case "linux":
		return []CommandSpec{
			{Name: "fusermount3", Args: []string{"-u", mountpoint}},
			{Name: "fusermount3", Args: []string{"-u", "-z", mountpoint}},
			{Name: "fusermount", Args: []string{"-u", mountpoint}},
			{Name: "fusermount", Args: []string{"-u", "-z", mountpoint}},
			{Name: "umount", Args: []string{mountpoint}},
			{Name: "umount", Args: []string{"-l", mountpoint}},
			{Name: "umount", Args: []string{"-f", mountpoint}},
			{Name: "umount", Args: []string{"-l", "-f", mountpoint}},
		}, nil
	case "darwin":
		return []CommandSpec{
			{Name: "umount", Args: []string{mountpoint}},
			{Name: "umount", Args: []string{"-f", mountpoint}},
			{Name: "diskutil", Args: []string{"unmount", "force", mountpoint}},
		}, nil
	case "windows":
		return []CommandSpec{
			{Name: "umount", Args: []string{"-f", mountpoint}},
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

func waitForMountActive(goos, mountpoint string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		active, err := isMountActive(goos, mountpoint)
		if err == nil && active {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("mount did not become active: %s", mountpoint)
}

func isMountActive(goos, mountpoint string) (bool, error) {
	switch goos {
	case "linux":
		file, err := os.Open("/proc/self/mountinfo")
		if err != nil {
			return false, err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) > 4 && fields[4] == mountpoint {
				return true, nil
			}
		}
		return false, scanner.Err()
	case "darwin":
		return isDarwinMountActive(mountpoint)
	case "windows":
		target := mountpoint
		if regexp.MustCompile(`^[A-Za-z]:$`).MatchString(target) {
			target += `\`
		}
		_, err := os.Stat(target)
		if err == nil {
			return true, nil
		}
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	default:
		return false, nil
	}
}

func resolveMountPath(path string) string {
	path = filepath.Clean(path)
	if path == "" {
		return path
	}

	if resolved, err := filepath.EvalSymlinks(path); err == nil && resolved != "" {
		return filepath.Clean(resolved)
	}

	volume := filepath.VolumeName(path)
	root := string(os.PathSeparator)
	if volume != "" {
		root = volume + root
	}

	current := path
	var suffix []string
	for {
		if current == "" || current == "." {
			break
		}

		if resolved, err := filepath.EvalSymlinks(current); err == nil && resolved != "" {
			base := filepath.Clean(resolved)
			if len(suffix) == 0 {
				return base
			}
			parts := append([]string{base}, suffix...)
			return filepath.Join(parts...)
		}

		if current == root {
			break
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		suffix = append([]string{filepath.Base(current)}, suffix...)
		current = parent
	}

	return path
}
