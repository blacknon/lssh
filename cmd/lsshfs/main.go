// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"os"

	lsshfs "github.com/blacknon/lssh/internal/app/lsshfs"
	"github.com/blacknon/lssh/internal/common"
)

func main() {
	app := lsshfs.Lsshfs()
	args := common.NormalizeGenerateLSSHConfArgs(os.Args)
	args = common.NormalizeTrailingBoolFlags(args, map[string]bool{
		"--foreground": true,
		"--rw":         true,
		"--list":       true,
		"-l":           true,
		"--help":       true,
		"-h":           true,
	}, map[string]bool{
		"-H":                   true,
		"--host":               true,
		"-F":                   true,
		"--file":               true,
		"--generate-lssh-conf": true,
	})
	args = common.ParseArgs(app.Flags, args)
	app.Run(args)
}
