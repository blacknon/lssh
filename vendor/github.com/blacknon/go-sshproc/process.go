// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	"path/filepath"
	"strconv"

	proc "github.com/c9s/goprocinfo/linux"
)

func (p *ConnectWithProc) ReadProcess(pid uint64, path string) (*proc.Process, error) {
	var err error
	pp := filepath.Join(path, strconv.FormatUint(pid, 10))

	if _, err = p.sftp.Stat(pp); err != nil {
		return nil, err
	}

	process := proc.Process{}

	var io *proc.ProcessIO
	var stat *proc.ProcessStat
	var statm *proc.ProcessStatm
	var status *proc.ProcessStatus
	var cmdline string

	if io, err = p.ReadProcessIO(filepath.Join(pp, "io")); err != nil {
		return nil, err
	}

	if stat, err = p.ReadProcessStat(filepath.Join(pp, "stat")); err != nil {
		return nil, err
	}

	if statm, err = p.ReadProcessStatm(filepath.Join(pp, "statm")); err != nil {
		return nil, err
	}

	if status, err = p.ReadProcessStatus(filepath.Join(pp, "status")); err != nil {
		return nil, err
	}

	if cmdline, err = p.ReadProcessCmdline(filepath.Join(pp, "cmdline")); err != nil {
		return nil, err
	}

	process.IO = *io
	process.Stat = *stat
	process.Statm = *statm
	process.Status = *status
	process.Cmdline = cmdline

	return &process, nil
}

func (p *ConnectWithProc) ReadProcessPassPermission(pid uint64, path string) (*proc.Process, error) {
	var err error
	pp := filepath.Join(path, strconv.FormatUint(pid, 10))

	if _, err = p.sftp.Stat(pp); err != nil {
		return nil, err
	}

	process := proc.Process{}

	var io *proc.ProcessIO
	var stat *proc.ProcessStat
	var statm *proc.ProcessStatm
	var status *proc.ProcessStatus
	var cmdline string

	var xerr error

	if io, err = p.ReadProcessIO(filepath.Join(pp, "io")); err != nil {
		xerr = err
	}

	if stat, err = p.ReadProcessStat(filepath.Join(pp, "stat")); err != nil {
		xerr = err
	}

	if statm, err = p.ReadProcessStatm(filepath.Join(pp, "statm")); err != nil {
		xerr = err
	}

	if status, err = p.ReadProcessStatus(filepath.Join(pp, "status")); err != nil {
		xerr = err
	}

	if cmdline, err = p.ReadProcessCmdline(filepath.Join(pp, "cmdline")); err != nil {
		xerr = err
	}

	if io != nil {
		process.IO = *io
	}

	if stat != nil {
		process.Stat = *stat
	}

	if statm != nil {
		process.Statm = *statm
	}

	if status != nil {
		process.Status = *status
	}

	if cmdline != "" {
		process.Cmdline = cmdline
	}

	return &process, xerr
}
