// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	proc "github.com/c9s/goprocinfo/linux"
)

func (p *ConnectWithProc) ReadDisk(path string) (*proc.Disk, error) {
	fs, err := p.sftp.StatVFS(path)
	if err != nil {
		return nil, err
	}
	disk := proc.Disk{}
	disk.All = fs.Blocks * uint64(fs.Bsize)
	disk.Free = fs.Bfree * uint64(fs.Bsize)
	disk.Used = disk.All - disk.Free
	disk.FreeInodes = fs.Ffree
	return &disk, nil
}
