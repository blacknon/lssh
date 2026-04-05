// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	"bufio"
	"strings"

	proc "github.com/c9s/goprocinfo/linux"
)

func (p *ConnectWithProc) ReadMounts(path string) (*proc.Mounts, error) {
	fin, err := p.sftp.Open(path)
	if err != nil {
		return nil, err
	}
	defer fin.Close()

	var mounts = proc.Mounts{}

	scanner := bufio.NewScanner(fin)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		var mount = &proc.Mount{
			fields[0],
			fields[1],
			fields[2],
			fields[3],
		}
		mounts.Mounts = append(mounts.Mounts, *mount)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &mounts, nil
}
