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

func (p *ConnectWithProc) ReadNetUnixDomainSockets(fpath string) (*proc.NetUnixDomainSockets, error) {
	file, err := p.sftp.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(b), "\n")
	unixDomainSockets := &proc.NetUnixDomainSockets{}

	for i := 1; i < len(lines); i++ {
		line := lines[i]
		f := strings.Fields(line)

		if len(f) < 8 {
			continue
		}

		socket := proc.NetUnixDomainSocket{}
		if socket.RefCount, err = strconv.ParseUint(f[1], 16, 64); err != nil {
			return nil, errors.New("Cannot parse unix domain socket [invalid RefCount]: " + f[1])
		}

		if socket.Protocol, err = strconv.ParseUint(f[2], 10, 64); err != nil {
			return nil, errors.New("Cannot parse unix domain socket [invalid Protocol]: " + f[2])
		}

		if socket.Flags, err = strconv.ParseUint(f[3], 10, 64); err != nil {
			return nil, errors.New("Cannot parse unix domain socket [invalid Flags]: " + f[3])
		}

		if socket.Type, err = strconv.ParseUint(f[4], 10, 64); err != nil {
			return nil, errors.New("Cannot parse unix domain socket [invalid Type]: " + f[4])
		}

		if socket.State, err = strconv.ParseUint(f[5], 10, 64); err != nil {
			return nil, errors.New("Cannot parse unix domain socket [invalid State]: " + f[5])
		}

		if socket.Inode, err = strconv.ParseUint(f[6], 10, 64); err != nil {
			return nil, errors.New("Cannot parse unix domain socket [invalid Inode]: " + f[6])
		}

		socket.Path = f[7]
		unixDomainSockets.Sockets = append(unixDomainSockets.Sockets, socket)
	}
	return unixDomainSockets, nil
}
