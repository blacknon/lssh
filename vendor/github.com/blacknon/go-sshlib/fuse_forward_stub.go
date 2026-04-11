//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd)

// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import "errors"

func (c *Connect) FUSEForward(mountpoint, basepoint string) error {
	return errors.New("sshlib: FUSEForward is not supported on this platform")
}

func (c *Connect) FUSEReverseForward(mountpoint, sharepoint string) error {
	return errors.New("sshlib: FUSEReverseForward is not supported on this platform")
}
