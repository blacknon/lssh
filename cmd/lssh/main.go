// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"os"

	lssh "github.com/blacknon/lssh/internal/app/lssh"
	"github.com/blacknon/lssh/internal/common"
)

func main() {
	app := lssh.Lssh()
	args := common.ParseArgs(app.Flags, os.Args)
	app.Run(args)
}
