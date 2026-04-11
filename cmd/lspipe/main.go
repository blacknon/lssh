// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/blacknon/lssh/internal/app/lspipe"
	"github.com/blacknon/lssh/internal/common"
)

func main() {
	app := lspipe.Lspipe()
	args := common.ParseArgs(app.Flags, common.NormalizeGenerateLSSHConfArgs(os.Args))
	if err := app.Run(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
