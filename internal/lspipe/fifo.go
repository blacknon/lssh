// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lspipe

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type FIFORecord struct {
	SessionName string   `json:"session_name"`
	Name        string   `json:"name"`
	Dir         string   `json:"dir"`
	PID         int      `json:"pid"`
	Hosts       []string `json:"hosts"`
}

type FIFOEndpoint struct {
	Scope     string
	Hosts     []string
	Command   string
	StdinPath string
	CmdPath   string
	OutPath   string
}

func fifoRecordPath(sessionName, fifoName string) (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}

	sum := sha1.Sum([]byte("fifo:" + normalizeSessionName(sessionName) + ":" + normalizeFIFOName(fifoName)))
	filename := hex.EncodeToString(sum[:]) + ".fifo.json"
	return filepath.Join(dir, filename), nil
}

func fifoBaseDir(sessionName, fifoName string) (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "fifo", normalizeSessionName(sessionName), normalizeFIFOName(fifoName)), nil
}

func normalizeFIFOName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "default"
	}
	name = strings.ReplaceAll(name, string(filepath.Separator), "_")
	return name
}

func SaveFIFORecord(record FIFORecord) error {
	dir, err := stateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	record.SessionName = normalizeSessionName(record.SessionName)
	record.Name = normalizeFIFOName(record.Name)
	sort.Strings(record.Hosts)

	path, err := fifoRecordPath(record.SessionName, record.Name)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func LoadFIFORecord(sessionName, fifoName string) (FIFORecord, error) {
	path, err := fifoRecordPath(sessionName, fifoName)
	if err != nil {
		return FIFORecord{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return FIFORecord{}, err
	}

	var record FIFORecord
	if err := json.Unmarshal(data, &record); err != nil {
		return FIFORecord{}, err
	}
	record.SessionName = normalizeSessionName(record.SessionName)
	record.Name = normalizeFIFOName(record.Name)
	sort.Strings(record.Hosts)
	return record, nil
}

func RemoveFIFORecord(sessionName, fifoName string) error {
	path, err := fifoRecordPath(sessionName, fifoName)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func ListFIFORecords() ([]FIFORecord, error) {
	dir, err := stateDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []FIFORecord{}, nil
		}
		return nil, err
	}

	records := make([]FIFORecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".fifo.json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		var record FIFORecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}
		record.SessionName = normalizeSessionName(record.SessionName)
		record.Name = normalizeFIFOName(record.Name)
		sort.Strings(record.Hosts)
		records = append(records, record)
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].SessionName == records[j].SessionName {
			return records[i].Name < records[j].Name
		}
		return records[i].SessionName < records[j].SessionName
	})
	return records, nil
}

func BuildFIFOEndpoints(session Session, fifoName string) ([]FIFOEndpoint, string, error) {
	if runtime.GOOS == "windows" {
		return nil, "", fmt.Errorf("named pipes are not supported by lspipe on windows yet")
	}

	hosts := append([]string(nil), session.Hosts...)
	sort.Strings(hosts)

	baseDir, err := fifoBaseDir(session.Name, fifoName)
	if err != nil {
		return nil, "", err
	}

	endpoints := make([]FIFOEndpoint, 0, len(hosts)+1)
	endpoints = append(endpoints, buildFIFOEndpoint(baseDir, "all", hosts))
	for _, host := range hosts {
		endpoints = append(endpoints, buildFIFOEndpoint(baseDir, host, []string{host}))
	}
	return endpoints, baseDir, nil
}

func buildFIFOEndpoint(baseDir, scope string, hosts []string) FIFOEndpoint {
	hosts = append([]string(nil), hosts...)
	sort.Strings(hosts)
	scope = sanitizeScope(scope)
	return FIFOEndpoint{
		Scope:     scope,
		Hosts:     hosts,
		Command:   filepath.Join(baseDir, scope+".cmd"),
		StdinPath: filepath.Join(baseDir, scope+".stdin"),
		CmdPath:   filepath.Join(baseDir, scope+".cmd"),
		OutPath:   filepath.Join(baseDir, scope+".out"),
	}
}

func sanitizeScope(scope string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return "all"
	}
	scope = strings.ReplaceAll(scope, string(filepath.Separator), "_")
	return scope
}
