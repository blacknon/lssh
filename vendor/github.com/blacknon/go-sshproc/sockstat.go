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

func (p *ConnectWithProc) ReadSockStat(path string) (*proc.SockStat, error) {
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

	// Maps a meminfo metric to its value (i.e. MemTotal --> 100000)
	statMap := map[string]uint64{}

	var sockStat proc.SockStat = proc.SockStat{}

	for _, line := range lines {
		if strings.Index(line, ":") == -1 {
			continue
		}

		statType := line[0:strings.Index(line, ":")] + "."

		// The fields have this pattern: inuse 27 orphan 1 tw 23 alloc 31 mem 3
		// The stats are grouped into pairs and need to be parsed and placed into the stat map.
		key := ""
		for k, v := range strings.Fields(line[strings.Index(line, ":")+1:]) {
			// Every second field is a value.
			if (k+1)%2 != 0 {
				key = v
				continue
			}
			val, _ := strconv.ParseUint(v, 10, 64)
			statMap[statType+key] = val
		}
	}

	elem := reflect.ValueOf(&sockStat).Elem()
	typeOfElem := elem.Type()

	for i := 0; i < elem.NumField(); i++ {
		val, ok := statMap[typeOfElem.Field(i).Tag.Get("field")]
		if ok {
			elem.Field(i).SetUint(val)
		}
	}

	return &sockStat, nil
}
