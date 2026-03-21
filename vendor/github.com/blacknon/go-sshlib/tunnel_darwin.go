// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

//go:build darwin

package sshlib

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

const (
	utunControlName = "com.apple.net.utun_control"
	utunOptIfName   = 2
	sysprotoControl = 2
)

func openTunnelDevice(unit int, mode TunnelMode) (*tunnelDevice, error) {
	if mode != TunnelModePointToPoint {
		return nil, fmt.Errorf("tunnel mode %d is not supported on darwin", mode)
	}

	fd, err := unix.Socket(unix.AF_SYSTEM, unix.SOCK_DGRAM, sysprotoControl)
	if err != nil {
		return nil, fmt.Errorf("create utun control socket: %w", err)
	}

	var ctlInfo unix.CtlInfo
	copy(ctlInfo.Name[:], utunControlName)
	if err := unix.IoctlCtlInfo(fd, &ctlInfo); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("lookup utun control %q: %w", utunControlName, err)
	}

	addr := &unix.SockaddrCtl{
		ID:   ctlInfo.Id,
		Unit: utunControlUnit(unit),
	}
	if err := unix.Connect(fd, addr); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("connect utun control socket (unit=%s): %w", describeTunnelUnit(unit), err)
	}

	name, err := unix.GetsockoptString(fd, sysprotoControl, utunOptIfName)
	if err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("read utun interface name: %w", err)
	}

	if err := validateTunnelDevice(mode, name); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("validate darwin tunnel interface %q: %w", name, err)
	}

	actualUnit, err := parseTunnelInterfaceUnit(name)
	if err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("parse darwin tunnel interface unit from %q: %w", name, err)
	}

	file := os.NewFile(uintptr(fd), name)
	return &tunnelDevice{
		ReadWriteCloser: &utunDevice{file: file},
		Name:            name,
		Unit:            actualUnit,
	}, nil
}
