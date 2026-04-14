// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

//go:build linux

package sshlib

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

type linuxTunnelDevice struct {
	file *os.File
	fd   int
}

func openTunnelDevice(unit int, mode TunnelMode) (*tunnelDevice, error) {
	name := buildLinuxTunnelName(unit, mode)

	ifr, err := unix.NewIfreq(name)
	if err != nil {
		return nil, fmt.Errorf("prepare linux tunnel interface request %q: %w", name, err)
	}

	flags := uint16(unix.IFF_NO_PI)
	switch mode {
	case TunnelModePointToPoint:
		flags |= unix.IFF_TUN
	case TunnelModeEthernet:
		flags |= unix.IFF_TAP
	default:
		return nil, fmt.Errorf("unsupported tunnel mode: %d", mode)
	}
	ifr.SetUint16(flags)

	file, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return nil, formatLinuxTunnelOpenError(err)
	}

	if err := unix.IoctlIfreq(int(file.Fd()), unix.TUNSETIFF, ifr); err != nil {
		file.Close()
		return nil, fmt.Errorf("ioctl TUNSETIFF for %q: %w", name, err)
	}

	actualName := ifr.Name()
	if err := validateTunnelDevice(mode, actualName); err != nil {
		file.Close()
		return nil, fmt.Errorf("validate linux tunnel interface %q: %w", actualName, err)
	}

	actualUnit, err := parseTunnelInterfaceUnit(actualName)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("parse linux tunnel interface unit from %q: %w", actualName, err)
	}

	return &tunnelDevice{
		ReadWriteCloser: &linuxTunnelDevice{file: file, fd: int(file.Fd())},
		Name:            actualName,
		Unit:            actualUnit,
	}, nil
}

func (d *linuxTunnelDevice) Read(p []byte) (int, error) {
	for {
		n, err := unix.Read(d.fd, p)
		if err == nil {
			return n, nil
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		return 0, err
	}
}

func (d *linuxTunnelDevice) Write(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		n, err := unix.Write(d.fd, p[written:])
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			return written, err
		}
		written += n
	}
	return written, nil
}

func (d *linuxTunnelDevice) Close() error {
	if d.file == nil {
		return nil
	}
	err := d.file.Close()
	d.file = nil
	d.fd = -1
	return err
}

func formatLinuxTunnelOpenError(err error) error {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("open /dev/net/tun: %w (TUN/TAP device is unavailable; ensure the tun module is loaded and, in containers, pass /dev/net/tun and NET_ADMIN)", err)
	case errors.Is(err, os.ErrPermission):
		return fmt.Errorf("open /dev/net/tun: %w (permission denied; try running with sufficient privileges or grant CAP_NET_ADMIN)", err)
	default:
		return fmt.Errorf("open /dev/net/tun: %w", err)
	}
}
