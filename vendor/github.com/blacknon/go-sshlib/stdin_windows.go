// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.
//go:build windows
// +build windows

package sshlib

import (
	"io"

	windowsconsole "github.com/moby/term/windows"
	"golang.org/x/sys/windows"
)

func GetStdin() io.ReadCloser {
	h := uint32(windows.STD_INPUT_HANDLE)
	stdin := windowsconsole.NewAnsiReader(int(h))

	return stdin
}
