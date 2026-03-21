// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

//go:build !linux && !darwin

package sshlib

import (
	"fmt"
	"runtime"
)

func openTunnelDevice(unit int, mode TunnelMode) (*tunnelDevice, error) {
	return nil, fmt.Errorf("ssh tunnel forwarding is not supported on %s (local=%s, mode=%s)", runtime.GOOS, describeTunnelUnit(unit), mode.String())
}
