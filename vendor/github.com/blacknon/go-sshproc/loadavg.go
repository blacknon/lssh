// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	"errors"
	"io"
	"strconv"
	"strings"

	proc "github.com/c9s/goprocinfo/linux"
)

func (p *ConnectWithProc) ReadLoadAvg(path string) (*proc.LoadAvg, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(string(b))
	fields := strings.Fields(content)

	if len(fields) < 5 {
		return nil, errors.New("Cannot parse loadavg: " + content)
	}

	process := strings.Split(fields[3], "/")

	if len(process) != 2 {
		return nil, errors.New("Cannot parse loadavg: " + content)
	}

	loadavg := proc.LoadAvg{}

	if loadavg.Last1Min, err = strconv.ParseFloat(fields[0], 64); err != nil {
		return nil, err
	}

	if loadavg.Last5Min, err = strconv.ParseFloat(fields[1], 64); err != nil {
		return nil, err
	}

	if loadavg.Last15Min, err = strconv.ParseFloat(fields[2], 64); err != nil {
		return nil, err
	}

	if loadavg.ProcessRunning, err = strconv.ParseUint(process[0], 10, 64); err != nil {
		return nil, err
	}

	if loadavg.ProcessTotal, err = strconv.ParseUint(process[1], 10, 64); err != nil {
		return nil, err
	}

	if loadavg.LastPID, err = strconv.ParseUint(fields[4], 10, 64); err != nil {
		return nil, err
	}

	return &loadavg, nil
}
