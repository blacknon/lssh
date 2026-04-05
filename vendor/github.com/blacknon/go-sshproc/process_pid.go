// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func (p *ConnectWithProc) ReadMaxPID(path string) (uint64, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return 0, err
	}

	s := strings.TrimSpace(string(b))

	i, err := strconv.ParseUint(s, 10, 64)

	if err != nil {
		return 0, err
	}

	return i, nil

}

func (p *ConnectWithProc) ListPID(path string, max uint64) ([]uint64, error) {
	l := make([]uint64, 0, 5)

	for i := uint64(1); i <= max; i++ {

		pp := filepath.Join(path, strconv.FormatUint(i, 10))

		s, err := p.sftp.Stat(pp)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		if err != nil || !s.IsDir() {
			continue
		}

		l = append(l, i)

	}

	return l, nil
}

var pIDRegExp = regexp.MustCompile(`^\d+$`)

// In ListPID, the pid is obtained by looping the specified number, but since this is slow to handle via ssh.
// It be handled by extracting the path that only has numbers under `/proc`.
func (p *ConnectWithProc) ListInPID(path string) ([]uint64, error) {
	l := make([]uint64, 0, 5)

	fs, err := p.sftp.ReadDir(path)
	if err != nil {
		return l, err
	}

	for _, f := range fs {
		if pIDRegExp.MatchString(f.Name()) {
			i, err := strconv.ParseUint(f.Name(), 10, 64)
			if err != nil {
				return l, err
			}
			l = append(l, i)
		}
	}

	return l, nil
}
