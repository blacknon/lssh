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

type FileNr struct {
	Allocated uint64 `json:"allocated"`
	Unused    uint64 `json:"unused"`
	Max       uint64 `json:"max"`
}

func (p *ConnectWithProc) ReadUint64(path string) (uint64, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return 0, err
	}

	return parseUint64Data(data)
}

func (p *ConnectWithProc) ReadFileNr(path string) (*FileNr, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return parseFileNrData(data)
}

func parseUint64Data(data []byte) (uint64, error) {
	value := strings.TrimSpace(string(data))
	if value == "" {
		return 0, fmt.Errorf("scalar data is empty")
	}

	return strconv.ParseUint(value, 10, 64)
}

func parseFileNrData(data []byte) (*FileNr, error) {
	fields := strings.Fields(string(data))
	if len(fields) != 3 {
		return nil, fmt.Errorf("invalid file-nr data: %s", strings.TrimSpace(string(data)))
	}

	allocated, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return nil, err
	}
	unused, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return nil, err
	}
	max, err := strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		return nil, err
	}

	return &FileNr{
		Allocated: allocated,
		Unused:    unused,
		Max:       max,
	}, nil
}
