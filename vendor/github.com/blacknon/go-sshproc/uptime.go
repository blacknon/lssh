// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	"io"
	"strconv"
	"strings"

	proc "github.com/c9s/goprocinfo/linux"
)

func (p *ConnectWithProc) ReadUptime(path string) (*proc.Uptime, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(string(b))
	uptime := proc.Uptime{}
	if uptime.Total, err = strconv.ParseFloat(fields[0], 64); err != nil {
		return nil, err
	}
	if uptime.Idle, err = strconv.ParseFloat(fields[1], 64); err != nil {
		return nil, err
	}
	return &uptime, nil
}
