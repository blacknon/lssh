// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

type PressureMetrics struct {
	Avg10  float64 `json:"avg10"`
	Avg60  float64 `json:"avg60"`
	Avg300 float64 `json:"avg300"`
	Total  uint64  `json:"total"`
}

type PressureStat struct {
	Some *PressureMetrics `json:"some,omitempty"`
	Full *PressureMetrics `json:"full,omitempty"`
}

func (p *ConnectWithProc) ReadPressure(path string) (*PressureStat, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return parsePressureData(data)
}

func parsePressureData(data []byte) (*PressureStat, error) {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	stat := &PressureStat{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		name, metrics, err := parsePressureLine(line)
		if err != nil {
			return nil, err
		}

		switch name {
		case "some":
			stat.Some = metrics
		case "full":
			stat.Full = metrics
		default:
			return nil, fmt.Errorf("unknown pressure scope: %s", name)
		}
	}

	if stat.Some == nil && stat.Full == nil {
		return nil, fmt.Errorf("pressure data is empty")
	}

	return stat, nil
}

func parsePressureLine(line string) (string, *PressureMetrics, error) {
	fields := strings.Fields(line)
	if len(fields) != 5 {
		return "", nil, fmt.Errorf("invalid pressure line: %s", line)
	}

	metrics := &PressureMetrics{}
	for _, field := range fields[1:] {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			return "", nil, fmt.Errorf("invalid pressure metric: %s", field)
		}

		switch parts[0] {
		case "avg10":
			value, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				return "", nil, err
			}
			metrics.Avg10 = value
		case "avg60":
			value, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				return "", nil, err
			}
			metrics.Avg60 = value
		case "avg300":
			value, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				return "", nil, err
			}
			metrics.Avg300 = value
		case "total":
			value, err := strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				return "", nil, err
			}
			metrics.Total = value
		default:
			return "", nil, fmt.Errorf("unknown pressure metric: %s", parts[0])
		}
	}

	return fields[0], metrics, nil
}
