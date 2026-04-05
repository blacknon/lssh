// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	"io"
	"reflect"
	"strconv"
	"strings"

	proc "github.com/c9s/goprocinfo/linux"
)

func (p *ConnectWithProc) ReadProcessIO(path string) (*proc.ProcessIO, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Maps a io metric to its value (i.e. rchar --> 100000)
	m := map[string]uint64{}

	io := proc.ProcessIO{}

	lines := strings.Split(string(b), "\n")

	for _, line := range lines {

		if strings.Index(line, ": ") == -1 {
			continue
		}

		l := strings.Split(line, ": ")

		k := l[0]
		v, err := strconv.ParseUint(l[1], 10, 64)

		if err != nil {
			return nil, err
		}

		m[k] = v

	}

	e := reflect.ValueOf(&io).Elem()
	t := e.Type()

	for i := 0; i < e.NumField(); i++ {

		k := t.Field(i).Tag.Get("field")

		v, ok := m[k]

		if ok {
			e.Field(i).SetUint(v)
		}

	}

	return &io, nil
}
