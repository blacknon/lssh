// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"os"

	"github.com/blacknon/lssh/internal/app/lscp"
	"github.com/blacknon/lssh/internal/common"
)

func main() {
	app := lscp.Lscp()
	args := common.ParseArgs(app.Flags, common.NormalizeGenerateLSSHConfArgs(os.Args))
	app.Run(args)
}
