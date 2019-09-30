// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package main

import (
	"os"

	"github.com/blacknon/lssh/common"
)

func main() {
	app := Lscp()
	args := common.ParseArgs(app.Flags, os.Args)
	app.Run(args)
}
