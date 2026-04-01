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

func (p *ConnectWithProc) ReadSnmp(path string) (*proc.Snmp, error) {
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

	// Maps an SNMP metric to its value (i.e. SyncookiesSent --> 0)
	statMap := make(map[string]string)

	// patterns
	// Ip: Forwarding DefaultTTL InReceives InHdrErrors... <-- header
	// Ip: 2 64 9305753793 0 0 0 0 0... <-- values

	for i := 1; i < len(lines); i = i + 2 {
		headers := strings.Fields(lines[i-1][strings.Index(lines[i-1], ":")+1:])
		values := strings.Fields(lines[i][strings.Index(lines[i], ":")+1:])
		protocol := strings.Replace(strings.Fields(lines[i-1])[0], ":", "", -1)

		for j, header := range headers {
			var val string
			if len(values) > j {
				val = values[j]
			} else {
				val = "UNKNOWN"
			}

			statMap[protocol+header] = val
		}
	}

	var snmp proc.Snmp = proc.Snmp{}

	elem := reflect.ValueOf(&snmp).Elem()
	typeOfElem := elem.Type()

	for i := 0; i < elem.NumField(); i++ {
		if val, ok := statMap[typeOfElem.Field(i).Name]; ok {
			parsedVal, _ := strconv.ParseUint(val, 10, 64)
			elem.Field(i).SetUint(parsedVal)
		}
	}

	return &snmp, nil
}
