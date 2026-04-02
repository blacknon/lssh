// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"os"

	"github.com/blacknon/lssh/internal/app/lsmon"
	"github.com/blacknon/lssh/internal/common"
)

func main() {
	app := lsmon.Lsmon()
	args := common.ParseArgs(app.Flags, os.Args)
	app.Run(args)
}
