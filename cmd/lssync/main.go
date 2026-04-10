package main

import (
	"os"

	"github.com/blacknon/lssh/internal/app/lssync"
	"github.com/blacknon/lssh/internal/common"
)

func main() {
	app := lssync.Lssync()
	args := common.ParseArgs(app.Flags, common.NormalizeGenerateLSSHConfArgs(os.Args))
	app.Run(args)
}
