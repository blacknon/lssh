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

var processStatRegExp = regexp.MustCompile("^(\\d+)( \\(.*?\\) )(.*)$")

func (p *ConnectWithProc) ReadProcessStat(path string) (*proc.ProcessStat, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	s := string(b)

	f := make([]string, 0, 32)

	e := processStatRegExp.FindStringSubmatch(strings.TrimSpace(s))

	// Inject process Pid
	f = append(f, e[1])

	// Inject process Comm
	f = append(f, strings.TrimSpace(e[2]))

	// Inject all remaining process info
	f = append(f, (strings.Fields(e[3]))...)

	stat := proc.ProcessStat{}

	for i := 0; i < len(f); i++ {
		switch i {
		case 0:
			if stat.Pid, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 1:
			stat.Comm = f[i]
		case 2:
			stat.State = f[i]
		case 3:
			if stat.Ppid, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 4:
			if stat.Pgrp, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 5:
			if stat.Session, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 6:
			if stat.TtyNr, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 7:
			if stat.Tpgid, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 8:
			if stat.Flags, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 9:
			if stat.Minflt, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 10:
			if stat.Cminflt, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 11:
			if stat.Majflt, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 12:
			if stat.Cmajflt, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 13:
			if stat.Utime, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 14:
			if stat.Stime, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 15:
			if stat.Cutime, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 16:
			if stat.Cstime, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 17:
			if stat.Priority, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 18:
			if stat.Nice, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 19:
			if stat.NumThreads, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 20:
			if stat.Itrealvalue, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 21:
			if stat.Starttime, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 22:
			if stat.Vsize, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 23:
			if stat.Rss, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 24:
			if stat.Rsslim, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 25:
			if stat.Startcode, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 26:
			if stat.Endcode, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 27:
			if stat.Startstack, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 28:
			if stat.Kstkesp, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 29:
			if stat.Kstkeip, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 30:
			if stat.Signal, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 31:
			if stat.Blocked, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 32:
			if stat.Sigignore, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 33:
			if stat.Sigcatch, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 34:
			if stat.Wchan, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 35:
			if stat.Nswap, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 36:
			if stat.Cnswap, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 37:
			if stat.ExitSignal, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 38:
			if stat.Processor, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 39:
			if stat.RtPriority, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 40:
			if stat.Policy, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 41:
			if stat.DelayacctBlkioTicks, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 42:
			if stat.GuestTime, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 43:
			if stat.CguestTime, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 44:
			if stat.StartData, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 45:
			if stat.EndData, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 46:
			if stat.StartBrk, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 47:
			if stat.ArgStart, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 48:
			if stat.ArgEnd, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 49:
			if stat.EnvStart, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 50:
			if stat.EnvEnd, err = strconv.ParseUint(f[i], 10, 64); err != nil {
				return nil, err
			}
		case 51:
			if stat.ExitCode, err = strconv.ParseInt(f[i], 10, 64); err != nil {
				return nil, err
			}
		}
	}

	return &stat, nil
}
