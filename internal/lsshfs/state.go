// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsshfs

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type MountRecord struct {
	Host       string `json:"host"`
	RemotePath string `json:"remote_path"`
	MountPoint string `json:"mount_point"`
	Backend    string `json:"backend"`
	PID        int    `json:"pid"`
	ReadWrite  bool   `json:"read_write"`
}

func stateDir() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CACHE_HOME")); xdg != "" {
		return filepath.Join(xdg, "lssh", "lsshfs"), nil
	}

	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "lssh", "lsshfs"), nil
}

func stateFilePath(mountpoint string) (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}

	sum := sha1.Sum([]byte(strings.ToLower(mountpoint)))
	name := hex.EncodeToString(sum[:]) + ".json"
	return filepath.Join(dir, name), nil
}

func StateFilePath(mountpoint string) (string, error) {
	return stateFilePath(mountpoint)
}

func writeMountRecord(record MountRecord) error {
	dir, err := stateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	path, err := stateFilePath(record.MountPoint)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

func removeMountRecord(mountpoint string) error {
	path, err := stateFilePath(mountpoint)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func RemoveMountRecord(mountpoint string) error {
	return removeMountRecord(mountpoint)
}

func loadMountRecord(mountpoint string) (MountRecord, error) {
	path, err := stateFilePath(mountpoint)
	if err != nil {
		return MountRecord{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return MountRecord{}, err
	}

	var record MountRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return MountRecord{}, err
	}
	return record, nil
}

func LoadMountRecord(mountpoint string) (MountRecord, error) {
	return loadMountRecord(mountpoint)
}

func listMountRecords() ([]MountRecord, error) {
	dir, err := stateDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []MountRecord{}, nil
		}
		return nil, err
	}

	records := make([]MountRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var record MountRecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}
		records = append(records, record)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].MountPoint < records[j].MountPoint
	})

	return records, nil
}

func ListMountRecords() ([]MountRecord, error) {
	return listMountRecords()
}

func parseRemoteSpec(value string) (host, remotePath string, err error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", fmt.Errorf("remote path is required")
	}

	if strings.HasPrefix(value, "@") {
		pair := strings.SplitN(value[1:], ":", 2)
		if len(pair) != 2 || strings.TrimSpace(pair[0]) == "" || strings.TrimSpace(pair[1]) == "" {
			return "", "", fmt.Errorf("invalid remote path format: expected @host:/path")
		}
		return strings.TrimSpace(pair[0]), strings.TrimSpace(pair[1]), nil
	}

	hosts, path := parseLegacyHostPath(value)
	if len(hosts) > 1 {
		return "", "", fmt.Errorf("lsshfs only supports a single host")
	}
	if len(hosts) == 1 {
		return hosts[0], path, nil
	}
	return "", path, nil
}

func ParseRemoteSpec(value string) (host, remotePath string, err error) {
	return parseRemoteSpec(value)
}

func parseLegacyHostPath(value string) ([]string, string) {
	if !strings.Contains(value, ":") {
		return nil, value
	}

	pair := strings.SplitN(value, ":", 2)
	if strings.HasPrefix(pair[1], "/") && pair[0] != "" {
		return []string{pair[0]}, pair[1]
	}

	return nil, value
}
