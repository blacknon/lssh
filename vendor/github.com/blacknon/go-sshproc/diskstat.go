// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	"io"
	"strconv"
	"strings"

	proc "github.com/c9s/goprocinfo/linux"
)

// ReadDiskStats reads and parses the file.
//
// Note:
// * Assumes a well formed file and will panic if it isn't.
func (p *ConnectWithProc) ReadDiskStats(path string) ([]proc.DiskStat, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	devices := strings.Split(string(b), "\n")
	results := make([]proc.DiskStat, len(devices)-1)

	for i := range results {
		fields := strings.Fields(devices[i])
		Major, _ := strconv.ParseInt(fields[0], 10, strconv.IntSize)
		results[i].Major = int(Major)
		Minor, _ := strconv.ParseInt(fields[1], 10, strconv.IntSize)
		results[i].Minor = int(Minor)
		results[i].Name = fields[2]
		results[i].ReadIOs, _ = strconv.ParseUint(fields[3], 10, 64)
		results[i].ReadMerges, _ = strconv.ParseUint(fields[4], 10, 64)
		results[i].ReadSectors, _ = strconv.ParseUint(fields[5], 10, 64)
		results[i].ReadTicks, _ = strconv.ParseUint(fields[6], 10, 64)
		results[i].WriteIOs, _ = strconv.ParseUint(fields[7], 10, 64)
		results[i].WriteMerges, _ = strconv.ParseUint(fields[8], 10, 64)
		results[i].WriteSectors, _ = strconv.ParseUint(fields[9], 10, 64)
		results[i].WriteTicks, _ = strconv.ParseUint(fields[10], 10, 64)
		results[i].InFlight, _ = strconv.ParseUint(fields[11], 10, 64)
		results[i].IOTicks, _ = strconv.ParseUint(fields[12], 10, 64)
		results[i].TimeInQueue, _ = strconv.ParseUint(fields[13], 10, 64)
	}

	return results, nil
}
