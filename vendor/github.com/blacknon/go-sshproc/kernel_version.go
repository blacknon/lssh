// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	"io"
)

func (p *ConnectWithProc) ReadData(path string) (string, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	return string(b), err
}
