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

func (p *ConnectWithProc) ReadNetStat(path string) (*proc.NetStat, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")

	// Maps a netstat metric to its value (i.e. SyncookiesSent --> 0)
	statMap := make(map[string]string)

	// patterns
	// TcpExt: SyncookiesSent SyncookiesRecv SyncookiesFailed... <-- header
	// TcpExt: 0 0 1764... <-- values

	for i := 1; i < len(lines); i = i + 2 {
		headers := strings.Fields(lines[i-1][strings.Index(lines[i-1], ":")+1:])
		values := strings.Fields(lines[i][strings.Index(lines[i], ":")+1:])

		for j, header := range headers {
			statMap[header] = values[j]
		}
	}

	var netstat proc.NetStat = proc.NetStat{}

	elem := reflect.ValueOf(&netstat).Elem()
	typeOfElem := elem.Type()

	for i := 0; i < elem.NumField(); i++ {
		if val, ok := statMap[typeOfElem.Field(i).Name]; ok {
			parsedVal, _ := strconv.ParseUint(val, 10, 64)
			elem.Field(i).SetUint(parsedVal)
		}
	}

	return &netstat, nil
}
