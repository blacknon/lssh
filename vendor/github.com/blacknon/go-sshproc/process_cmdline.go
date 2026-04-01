// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	"io"
	"strings"
)

func (p *ConnectWithProc) ReadProcessCmdline(path string) (string, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	if err != nil {
		return "", err
	}

	l := len(b) - 1 // Define limit before last byte ('\0')
	z := byte(0)    // '\0' or null byte
	s := byte(0x20) // space byte
	c := 0          // cursor of useful bytes

	for i := 0; i < l; i++ {

		// Check if next byte is not a '\0' byte.
		if b[i+1] != z {

			// Offset must match a '\0' byte.
			c = i + 2

			// If current byte is '\0', replace it with a space byte.
			if b[i] == z {
				b[i] = s
			}
		}
	}

	x := strings.TrimSpace(string(b[0:c]))

	return x, nil
}
