// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"os"

	"github.com/blacknon/lssh/internal/app/lsshell"
	"github.com/blacknon/lssh/internal/common"
)

func main() {
	app := lsshell.Lsshell()
	args := common.ParseArgs(app.Flags, common.NormalizeGenerateLSSHConfArgs(os.Args))
	app.Run(args)
}
