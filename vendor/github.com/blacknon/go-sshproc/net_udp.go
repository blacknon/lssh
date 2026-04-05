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

func (p *ConnectWithProc) ReadNetUDPSockets(path string, ip proc.NetIPDecoder) (*proc.NetUDPSockets, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(b), "\n")

	udp := &proc.NetUDPSockets{}

	for i := 1; i < len(lines); i++ {

		line := lines[i]

		f := strings.Fields(line)

		if len(f) < 13 {
			continue
		}

		s, err := parseNetSocket(f, ip)

		if err != nil {
			return nil, err
		}

		e := &proc.NetUDPSocket{
			NetSocket: *s,
			Drops:     0,
		}

		if e.Drops, err = strconv.ParseUint(f[12], 10, 64); err != nil {
			return nil, err
		}

		udp.Sockets = append(udp.Sockets, *e)
	}

	return udp, nil
}
