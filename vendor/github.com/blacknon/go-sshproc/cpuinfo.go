// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	"io"
	"regexp"
	"strconv"
	"strings"

	proc "github.com/c9s/goprocinfo/linux"
)

var cpuinfoRegExp = regexp.MustCompile("([^:]*?)\\s*:\\s*(.*)$")

func (p *ConnectWithProc) ReadCPUInfo(path string) (*proc.CPUInfo, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	content := string(b)
	lines := strings.Split(content, "\n")

	var cpuinfo = proc.CPUInfo{}
	var processor = &proc.Processor{CoreId: -1, PhysicalId: -1}

	for i, line := range lines {
		var key string
		var value string

		if len(line) == 0 && i != len(lines)-1 {
			// end of processor
			cpuinfo.Processors = append(cpuinfo.Processors, *processor)
			processor = &proc.Processor{}
			continue
		} else if i == len(lines)-1 {
			continue
		}

		submatches := cpuinfoRegExp.FindStringSubmatch(line)
		key = submatches[1]
		value = submatches[2]

		switch key {
		case "processor":
			processor.Id, _ = strconv.ParseInt(value, 10, 64)
		case "vendor_id":
			processor.VendorId = value
		case "model":
			processor.Model, _ = strconv.ParseInt(value, 10, 64)
		case "model name":
			processor.ModelName = value
		case "flags":
			processor.Flags = strings.Fields(value)
		case "cpu cores":
			processor.Cores, _ = strconv.ParseInt(value, 10, 64)
		case "cpu MHz":
			processor.MHz, _ = strconv.ParseFloat(value, 64)
		case "cache size":
			processor.CacheSize, _ = strconv.ParseInt(value[:strings.IndexAny(value, " \t\n")], 10, 64)
			if strings.HasSuffix(line, "MB") {
				processor.CacheSize *= 1024
			}
		case "physical id":
			processor.PhysicalId, _ = strconv.ParseInt(value, 10, 64)
		case "core id":
			processor.CoreId, _ = strconv.ParseInt(value, 10, 64)
		}
		/*
			processor	: 0
			vendor_id	: GenuineIntel
			cpu family	: 6
			model		: 26
			model name	: Intel(R) Xeon(R) CPU           L5520  @ 2.27GHz
		*/
	}
	return &cpuinfo, nil
}
