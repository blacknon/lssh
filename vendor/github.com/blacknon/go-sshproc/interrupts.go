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

func (p *ConnectWithProc) ReadInterrupts(path string) (*proc.Interrupts, error) {
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
	cpus := lines[0]
	lines = append(lines[:0], lines[1:]...)
	numCpus := len(strings.Fields(cpus))
	interrupts := make([]proc.Interrupt, 0)
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		counts := make([]uint64, 0)
		i := 0
		for ; i < numCpus; i++ {
			if len(fields) <= i+1 {
				break
			}
			count, err := strconv.ParseInt(fields[i+1], 10, 64)
			if err != nil {
				return nil, err
			}
			counts = append(counts, uint64(count))
		}
		name := strings.TrimSuffix(fields[0], ":")
		description := strings.Join(fields[i+1:], " ")
		interrupts = append(interrupts, proc.Interrupt{
			Name:        name,
			Counts:      counts,
			Description: description,
		})
	}
	return &proc.Interrupts{Interrupts: interrupts}, nil
}
