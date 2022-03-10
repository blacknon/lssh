// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.
//go:build !windows && !plan9 && !nacl
// +build !windows,!plan9,!nacl

package sshlib

import (
	"io"
	"os"
)

func GetStdin() io.ReadCloser {
	return os.Stdin
}
