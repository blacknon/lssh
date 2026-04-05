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

func (p *ConnectWithProc) ReadNetTCPSockets(path string, ip proc.NetIPDecoder) (*proc.NetTCPSockets, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimRight(string(b), "\n "), "\n")

	tcp := &proc.NetTCPSockets{}

	for i := 1; i < len(lines); i++ {

		line := lines[i]

		f := strings.Fields(line)

		s, err := parseNetSocket(f, ip)

		if err != nil {
			return nil, err
		}

		var n int64
		e := &proc.NetTCPSocket{
			NetSocket: *s,
		}

		// Depending on socket state, the number of fields presented is either
		// 12 or 17.
		if len(f) >= 17 {
			if e.RetransmitTimeout, err = strconv.ParseUint(f[12], 10, 64); err != nil {
				return nil, err
			}

			if e.PredictedTick, err = strconv.ParseUint(f[13], 10, 64); err != nil {
				return nil, err
			}

			if n, err = strconv.ParseInt(f[14], 10, 8); err != nil {
				return nil, err
			}
			e.AckQuick = uint8(n >> 1)
			e.AckPingpong = ((n & 1) == 1)

			if e.SendingCongestionWindow, err = strconv.ParseUint(f[15], 10, 64); err != nil {
				return nil, err
			}

			if e.SlowStartSizeThreshold, err = strconv.ParseInt(f[16], 10, 32); err != nil {
				return nil, err
			}
		}

		tcp.Sockets = append(tcp.Sockets, *e)
	}

	return tcp, nil
}
